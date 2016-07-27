package himawari

import (
	"image/png"
	"net/http"
	"os"
	"testing"
)

func TestTime(t *testing.T) {
	latest, err := Latest()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(latest)
	offset := TimeWithOffset(latest)
	t.Log(offset)
}

func TestChunkUrl(t *testing.T) {
	latest, err := Latest()
	if err != nil {
		t.Fatal(err)
	}
	ot := TimeWithOffset(latest)
	url := ChunkUrl(ot, DefaultLevel, Width, 1, 2)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	} else if resp.StatusCode != http.StatusOK {
		t.Fatal("status:", resp.Status, resp.StatusCode)
	}
	resp.Body.Close()
}

func TestLatestImage(t *testing.T) {
	img, err := LatestImage(0, true)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create("latest.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err = png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}
