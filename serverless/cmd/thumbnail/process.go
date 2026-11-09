package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // реєструє декодер JPEG
	"image/png"
	"log/slog"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

const thumbSize = 200

// process обробляє всі записи події. Кілька нотифікацій можуть прийти в одному
// виклику, тому ЗАВЖДИ ітеруємо Records (слайд 21).
//
// Ідемпотентність (слайди 21, 23, 46): ключ результату детермінований, тож
// повторна доставка тієї самої події просто перезапише той самий об'єкт.
//
// Повернення error запускає політику повторних спроб Lambda — тому помилки
// ТИМЧАСОВІ (S3 Get/Put) повертаємо, а ПОСТІЙНІ (файл — не зображення) лише
// логуємо: повтор нічого не виправить.
func process(ctx context.Context, log *slog.Logger, store ObjectStore, outBucket string, ev events.S3Event) error {
	for _, rec := range ev.Records {
		bucket := rec.S3.Bucket.Name
		// У S3-подіях ключ приходить URL-закодованим («my+cover.jpg», «%D0%BA…»).
		key, err := url.QueryUnescape(rec.S3.Object.Key)
		if err != nil {
			log.WarnContext(ctx, "skip: bad key encoding", "key", rec.S3.Object.Key, "error", err)
			continue
		}

		raw, err := store.Get(ctx, bucket, key)
		if err != nil {
			return fmt.Errorf("get %s/%s: %w", bucket, key, err)
		}

		img, format, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			log.WarnContext(ctx, "skip: not a supported image", "bucket", bucket, "key", key, "error", err)
			continue
		}

		thumb := fit(img, thumbSize, thumbSize)
		var buf bytes.Buffer
		if err := png.Encode(&buf, thumb); err != nil {
			return fmt.Errorf("encode png: %w", err)
		}

		outKey := thumbKey(key)
		if err := store.Put(ctx, outBucket, outKey, buf.Bytes(), "image/png"); err != nil {
			return fmt.Errorf("put %s/%s: %w", outBucket, outKey, err)
		}

		log.InfoContext(ctx, "thumbnail created",
			"source", bucket+"/"+key, "target", outBucket+"/"+outKey,
			"format", format,
			"from", fmt.Sprintf("%dx%d", img.Bounds().Dx(), img.Bounds().Dy()),
			"to", fmt.Sprintf("%dx%d", thumb.Bounds().Dx(), thumb.Bounds().Dy()))
	}
	return nil
}

// thumbKey: covers/dune.jpg → thumbs/covers/dune.png
func thumbKey(key string) string {
	ext := path.Ext(key)
	return "thumbs/" + strings.TrimSuffix(key, ext) + ".png"
}
