package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"coreheadlines/dynamo"
	"coreheadlines/economics"
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
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	headerFrom := os.Getenv("EMAIL_FROM_HEADER")
	envelopeFrom := os.Getenv("EMAIL_FROM_ADDRESS")
	toAddr := os.Getenv("EMAIL_TO")
	if smtpUser == "" || smtpPass == "" || headerFrom == "" || envelopeFrom == "" || toAddr == "" {
		return fmt.Errorf("one of SMTP_USER, SMTP_PASS, EMAIL_FROM_HEADER, EMAIL_FROM_ADDRESS, or EMAIL_TO is not set")
	}

	contactEmail := os.Getenv("BOT_EMAIL")
	userAgents := typesPkg.Agents{
		Bot: "CoreHeadlines/1.0 (+https://vitorio.us; " + contactEmail + ")",
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
					zap.String("source", fc.Source),
					zap.Error(err),
				)
				results[i].Err = err
				return
			}

			toPub, snippets, err := collectUnpublished(ctx, articles, db)
			if err != nil {
				logger.Error("Error collecting unpublished articles",
					zap.String("source", fc.Source),
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
			ctx,
			smtpHost, smtpPort,
			smtpUser, smtpPass,
			headerFrom, envelopeFrom, toAddr,
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

func getEconomics() string {
	var wg sync.WaitGroup

	var fedData []economics.EconomicsResponse
	var eurostatData []economics.EconomicsResponse

	wg.Add(2)

	go func() {
		defer wg.Done()
		fedData = economics.GetFED()
	}()

	go func() {
		defer wg.Done()
		eurostatData = economics.GetEurostat()
	}()

	wg.Wait()

	// Flatten everything into one slice
	var allDPs []economics.DataPoint
	for _, resp := range fedData {
		allDPs = append(allDPs, resp.DataPoints...)
	}
	for _, resp := range eurostatData {
		allDPs = append(allDPs, resp.DataPoints...)
	}

	// Sort by the Index field
	sort.Slice(allDPs, func(i, j int) bool {
		return allDPs[i].Index < allDPs[j].Index
	})

	// Build HTML items in sorted order
	var items []string
	for _, dp := range allDPs {
		items = append(items, fmt.Sprintf(
			`<div style="font-family:monospace; font-size:14px; margin:0;">%s (%s): %.2f%%</div>`,
			dp.Name, dp.Date, dp.Value,
		))
	}

	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, "\n")
}

func buildEmailHTML(items []string) string {
	sep := `<hr style="border:none;border-top:2px dashed #ccc;margin:12px 0;">`

	economicsContent := getEconomics()

	var bodyContent string

	if economicsContent != "" {
		bodyContent += sep + economicsContent
	}

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
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		logger.Info("Failed to load Europe/Madrid timezone, defaulting to UTC", zap.Error(err))
		loc = time.UTC
	}
	now := time.Now().In(loc)
	hour := now.Hour()
	if hour >= 1 && hour <= 7 && os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// Between 1:00 and 7:59, skip processing in Lambda
		return nil
	}

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
