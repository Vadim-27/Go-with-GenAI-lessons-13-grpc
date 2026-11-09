package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

type fakeStore struct {
	objects map[string][]byte
	getErr  error
	puts    int
}

func newFake() *fakeStore { return &fakeStore{objects: map[string][]byte{}} }

func (f *fakeStore) Get(_ context.Context, bucket, key string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	b, ok := f.objects[bucket+"/"+key]
	if !ok {
		return nil, errors.New("NoSuchKey")
	}
	return b, nil
}

func (f *fakeStore) Put(_ context.Context, bucket, key string, body []byte, _ string) error {
	f.puts++
	f.objects[bucket+"/"+key] = body
	return nil
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func s3Event(bucket string, keys ...string) events.S3Event {
	var ev events.S3Event
	for _, k := range keys {
		r := events.S3EventRecord{}
		r.S3.Bucket.Name = bucket
		r.S3.Object.Key = k
		ev.Records = append(ev.Records, r)
	}
	return ev
}

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestFitKeepsAspectRatio(t *testing.T) {
	got := fit(testImage(400, 200), 200, 200).Bounds()
	if got.Dx() != 200 || got.Dy() != 100 {
		t.Fatalf("got %dx%d, want 200x100", got.Dx(), got.Dy())
	}
	got = fit(testImage(100, 600), 200, 200).Bounds()
	if got.Dx() != 33 || got.Dy() != 200 {
		t.Fatalf("got %dx%d, want 33x200", got.Dx(), got.Dy())
	}
}

func TestFitDoesNotUpscale(t *testing.T) {
	got := fit(testImage(50, 40), 200, 200).Bounds()
	if got.Dx() != 50 || got.Dy() != 40 {
		t.Fatalf("small image must stay as is, got %dx%d", got.Dx(), got.Dy())
	}
}

func TestProcessCreatesThumbnail(t *testing.T) {
	st := newFake()
	st.objects["covers-in/covers/dune.png"] = encodePNG(t, testImage(800, 400))

	if err := process(context.Background(), quiet, st, "thumbs-out", s3Event("covers-in", "covers/dune.png")); err != nil {
		t.Fatal(err)
	}
	out, ok := st.objects["thumbs-out/thumbs/covers/dune.png"]
	if !ok {
		t.Fatalf("thumbnail not written; objects: %v", keys(st.objects))
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("thumbnail is %dx%d, want 200x100", b.Dx(), b.Dy())
	}
}

func TestProcessHandlesJPEGAndURLEncodedKey(t *testing.T) {
	var jb bytes.Buffer
	if err := jpeg.Encode(&jb, testImage(300, 300), nil); err != nil {
		t.Fatal(err)
	}
	st := newFake()
	st.objects["in/my cover.jpg"] = jb.Bytes()

	// S3 кодує пробіл як '+'
	if err := process(context.Background(), quiet, st, "out", s3Event("in", "my+cover.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.objects["out/thumbs/my cover.png"]; !ok {
		t.Fatalf("expected decoded key to be used; objects: %v", keys(st.objects))
	}
}

func TestProcessIsIdempotent(t *testing.T) {
	st := newFake()
	st.objects["in/a.png"] = encodePNG(t, testImage(500, 500))
	ev := s3Event("in", "a.png")

	for i := 0; i < 3; i++ { // at-least-once: та сама подія може прийти кілька разів
		if err := process(context.Background(), quiet, st, "out", ev); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(st.objects); n != 2 { // оригінал + рівно одна мініатюра
		t.Fatalf("repeated delivery must not create extra objects, have %d: %v", n, keys(st.objects))
	}
}

func TestProcessLoopsOverAllRecords(t *testing.T) {
	st := newFake()
	st.objects["in/a.png"] = encodePNG(t, testImage(300, 300))
	st.objects["in/b.png"] = encodePNG(t, testImage(300, 300))

	if err := process(context.Background(), quiet, st, "out", s3Event("in", "a.png", "b.png")); err != nil {
		t.Fatal(err)
	}
	if st.puts != 2 {
		t.Fatalf("puts = %d, want 2", st.puts)
	}
}

func TestPermanentFailureIsSkippedNotRetried(t *testing.T) {
	st := newFake()
	st.objects["in/readme.txt"] = []byte("definitely not an image")
	st.objects["in/ok.png"] = encodePNG(t, testImage(300, 300))

	err := process(context.Background(), quiet, st, "out", s3Event("in", "readme.txt", "ok.png"))
	if err != nil {
		t.Fatalf("non-image must not trigger Lambda retries, got: %v", err)
	}
	if _, ok := st.objects["out/thumbs/ok.png"]; !ok {
		t.Fatal("a bad record must not block the good ones after it")
	}
}

func TestTransientFailureIsReturnedForRetry(t *testing.T) {
	st := newFake()
	st.getErr = errors.New("connection reset")
	if err := process(context.Background(), quiet, st, "out", s3Event("in", "a.png")); err == nil {
		t.Fatal("transient S3 error must be returned so Lambda retries")
	}
}

func TestThumbKey(t *testing.T) {
	cases := map[string]string{
		"covers/dune.jpg": "thumbs/covers/dune.png",
		"a.png":           "thumbs/a.png",
		"noext":           "thumbs/noext.png",
	}
	for in, want := range cases {
		if got := thumbKey(in); got != want {
			t.Errorf("thumbKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func keys(m map[string][]byte) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
