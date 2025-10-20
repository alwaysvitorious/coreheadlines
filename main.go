package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"coreheadlines/dynamo"
	"coreheadlines/feeds"
	"coreheadlines/telegram"
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
) ([]typesPkg.MainStruct, error) {
	toPublish := make([]typesPkg.MainStruct, 0, len(articles))
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
	}
	return toPublish, nil
}

// *
// **
// ***
// ****
// ***** main
type feedResult struct {
	Articles []typesPkg.MainStruct
	Err      error
}

func runParsers(ctx context.Context, db *dynamodb.Client) error {
	email := os.Getenv("MAIN_EMAIL")
	if email == "" {
		return fmt.Errorf("MAIN_EMAIL not set")
	}

	userAgents := typesPkg.Agents{
		Bot:    "coreheadlines_bot/1.0 (+https://github.com/genbraham/coreheadlines; " + email + ")",
		Chrome: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
		Reader: "RSSReader/1.0 (+https://github.com/genbraham/coreheadlines; " + email + ")",
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

			toPub, err := collectUnpublished(ctx, articles, db)
			if err != nil {
				logger.Error("Error collecting unpublished articles",
					zap.String("source", fc.Header),
					zap.Error(err),
				)
				results[i].Err = err
				return
			}

			results[i].Articles = toPub
		}(idx, cfg)
	}

	wg.Wait()

	// Aggregate results preserving feed order
	allToPublish := make([]typesPkg.MainStruct, 0, 64)
	seen := make(map[string]bool, 256)

	for _, res := range results {
		if res.Err != nil {
			continue
		}
		for _, art := range res.Articles {
			if seen[art.GUID] {
				continue
			}
			seen[art.GUID] = true
			allToPublish = append(allToPublish, art)
		}
	}

	// Nothing new -> done
	if len(allToPublish) == 0 {
		return nil
	}

	// Send to telegram
	telegramBot := os.Getenv("TELEGRAM_BOT")
	if telegramBot == "" {
		return fmt.Errorf("TELEGRAM_BOT not set")
	}
	telegramChannel := os.Getenv("TELEGRAM_CHANNEL")
	if telegramChannel == "" {
		return fmt.Errorf("TELEGRAM_CHANNEL not set")
	}

	err := telegram.SendMessages(allToPublish, telegramBot, telegramChannel)
	if err != nil {
		logger.Error("Error sending messages",
			zap.Error(err),
		)
		return err
	}

	// Mark published
	if err := dynamo.BatchMarkPublished(ctx, db, allToPublish); err != nil {
		logger.Error("BatchMarkPublished failed after send",
			zap.Int("count", len(allToPublish)), zap.Error(err),
		)
		return err
	}

	logger.Info("Run complete", zap.Int("new_articles", len(allToPublish)))

	return nil
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
