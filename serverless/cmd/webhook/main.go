// Webhook-функція LibraTrack: приймає JSON про пожертву книги через API Gateway,
// валідує та нормалізує дані. Відповідає слайдам 18–19 та 41 лекції.
package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

// Логер створюється на рівні пакета — тобто під час init-фази (слайд 9, 23).
// Структурований JSON у stdout автоматично потрапляє в CloudWatch Logs.
var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// main() лише передає керування SDK — уся логіка в handler (слайд 18).
func main() {
	lambda.Start(handler)
}
