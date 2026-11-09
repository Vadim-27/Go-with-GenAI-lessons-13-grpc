// Lambda для створення мініатюр обкладинок книг LibraTrack.
// Тригер — S3 ObjectCreated (слайд 21). Клієнт S3 створюється ОДИН раз
// у фазі init і перевикористовується між викликами (слайди 9, 21, 23, 46).
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Ініціалізуються в init(): виконується лише під час холодного старту.
	store        ObjectStore
	outputBucket string
)

func init() {
	outputBucket = os.Getenv("OUTPUT_BUCKET")
	if outputBucket == "" {
		outputBucket = "libratrack-thumbnails"
	}

	s, err := newS3Store(context.Background())
	if err != nil {
		// Паніка в init → помилка фази ініціалізації; Lambda покаже її в логах.
		panic(err)
	}
	store = s
}

func handler(ctx context.Context, ev events.S3Event) error {
	return process(ctx, logger, store, outputBucket, ev)
}

func main() {
	lambda.Start(handler)
}
