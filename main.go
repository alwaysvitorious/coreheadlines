package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"coreheadlines/dynamo"
	"coreheadlines/email"
	"coreheadlines/feeds"
	"coreheadlines/tools"
	"coreheadlines/typesPkg"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// *
// **
// ***
// ****
// ***** logger
var logger *zap.Logger

func setupLogger() *zap.Logger {
	var core zapcore.Core
	var options []zap.Option

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "message"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	core = zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zap.InfoLevel,
	)

	options = append(options, zap.AddCaller())

	return zap.New(core, options...)
}

func init() {
	logger = setupLogger()
}

// *
// **
// ***
// ****
// ***** collect
func collectUnpublished(
	ctx context.Context,
	articles []typesPkg.MainStruct,
	db *dynamodb.Client,
) ([]typesPkg.MainStruct, []string, error) {
	var (
		toPublish []typesPkg.MainStruct
		snippets  []string
	)
	for _, art := range articles {
		pub, err := dynamo.IsArticlePublished(ctx, db, art.GUID)
		if err != nil {
			logger.Error("is-published check failed", zap.Error(err), zap.String("guid", art.GUID))
			continue
		}
		if pub {
			continue
		}
		toPublish = append(toPublish, art)
		if li := email.FormatPost(art); li != "" {
			snippets = append(snippets, li)
		}
	}
	return toPublish, snippets, nil
}

// *
// **
// ***
// ****
// ***** main
type feedResult struct {
	Articles []typesPkg.MainStruct
	Snippets []string
	Err      error
}

func runParsers(ctx context.Context, db *dynamodb.Client) error {
	// Load SMTP/email config
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		return fmt.Errorf("SMTP_HOST not set")
	}
	smtpPortStr := os.Getenv("SMTP_PORT")
	if smtpPortStr == "" {
		return fmt.Errorf("SMTP_PORT not set")
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP_PORT: %w", err)
	}
	stmpEmail := os.Getenv("MAIN_EMAIL")
	if stmpEmail == "" {
		return fmt.Errorf("MAIN_EMAIL not set")
	}
	smtpPass := os.Getenv("SMTP_PASS")
	if smtpPass == "" {
		return fmt.Errorf("SMTP_PASS not set")
	}

	userAgents := typesPkg.Agents{
		Bot: "CoreHeadlines/1.0 (+https://github.com/alwaysvitorious; " + stmpEmail + ")",
		Chrome: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
			"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",
		Reader: "Mozilla/5.0 (compatible; RSS Reader Bot 1.0)",
	}

	results := make([]feedResult, len(feeds.Feeds))
	var wg sync.WaitGroup

	for idx, cfg := range feeds.Feeds {
		wg.Add(1)
		go func(i int, fc feeds.FeedConfig) {
			defer wg.Done()

			articles, err := tools.ParseRSSFeed(ctx, userAgents, fc)
			if err != nil {
				logger.Error("Error parsing RSS feed",
					zap.String("url", fc.URL),
					zap.Error(err),
				)
				results[i].Err = err
				return
			}

			toPub, snippets, err := collectUnpublished(ctx, articles, db)
			if err != nil {
				logger.Error("Error collecting unpublished articles",
					zap.String("source", fc.Header),
					zap.Error(err),
				)
				results[i].Err = err
				return
			}

			results[i].Articles = toPub
			results[i].Snippets = snippets
		}(idx, cfg)
	}

	wg.Wait()

	// Aggregate results preserving feed order
	var (
		allToPublish []typesPkg.MainStruct
		allSnippets  []string
		seen         = make(map[string]bool)
	)

	for _, res := range results {
		if res.Err != nil {
			continue
		}

		for i, art := range res.Articles {
			if seen[art.GUID] {
				continue
			}
			seen[art.GUID] = true
			allToPublish = append(allToPublish, art)
			allSnippets = append(allSnippets, res.Snippets[i])
		}
	}

	// Nothing new -> done
	if len(allSnippets) == 0 {
		return nil
	}

	htmlBody := buildEmailHTML(allSnippets)

	// Send with 2 attempts max
	const maxRetries = 2
	var sendErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		sendErr = email.SendToEmail(
			smtpHost, smtpPort,
			stmpEmail, smtpPass,
			htmlBody,
		)
		if sendErr == nil {
			break
		}
		logger.Warn("SendToEmail failed, will retry",
			zap.Int("attempt", attempt),
			zap.Error(sendErr),
		)
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if sendErr != nil {
		logger.Error("Failed to send batch email after retries", zap.Error(sendErr))
		return sendErr
	}

	// Mark published
	if err := dynamo.BatchMarkPublished(ctx, db, allToPublish); err != nil {
		logger.Error("BatchMarkPublished failed after send",
			zap.Int("count", len(allToPublish)), zap.Error(err),
		)
		return err
	}

	return nil
}

func buildEmailHTML(items []string) string {
	sep := `<hr style="border:none;border-top:2px dashed #ccc;margin:12px 0;">`

	var bodyContent string

	bodyContent += sep + strings.Join(items, sep) + sep

	return `<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Core Headlines Update</title>
  </head>
  <body>` +
		bodyContent +
		`</body>
</html>`
}

func logic(ctx context.Context) error {
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	db := dynamodb.NewFromConfig(sdkConfig)

	return runParsers(ctx, db)
}

func main() {
	ctx := context.Background()
	defer logger.Sync()

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// Running in Lambda
		lambda.Start(func(ctx context.Context) error {
			return logic(ctx)
		})
	} else {
		// Running locally
		if err := godotenv.Load(); err != nil {
			logger.Warn("Failed to load .env file",
				zap.Error(err),
				zap.String("note", "This is expected in some environments"),
			)
		}

		if err := logic(ctx); err != nil {
			logger.Fatal("Application failed",
				zap.Error(err),
			)
		}
	}
}
