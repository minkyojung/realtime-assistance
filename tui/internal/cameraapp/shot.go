package cameraapp

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// 촬영 — 원본 한 장을 ~/Downloads 에 PNG 로 남긴다.
//
// 프리뷰(160x120)가 아니라 카메라 원본을 쓴다. 프리뷰 크기는 ASCII 로
// 줄일 것을 전제로 고른 값이라 사진으로는 남길 것이 없다.

// savePhoto 는 RGB 프레임을 파일로 쓰고 그 경로를 돌려준다.
func savePhoto(px []byte, w, h int, at time.Time) (string, error) {
	if w <= 0 || h <= 0 || len(px) < w*h*3 {
		return "", fmt.Errorf("프레임이 온전하지 않습니다: %dx%d, %d바이트", w, h, len(px))
	}

	dir, err := photoDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			// 화면에서 본 그대로 남긴다. 프리뷰가 거울이므로 사진도 거울이다 —
			// 방금 본 것과 좌우가 뒤집힌 사진이 나오면 그게 더 놀랍다.
			p := (y*w + (w - 1 - x)) * 3
			o := img.PixOffset(x, y)
			img.Pix[o] = px[p]
			img.Pix[o+1] = px[p+1]
			img.Pix[o+2] = px[p+2]
			img.Pix[o+3] = 0xff
		}
	}

	path := uniquePath(dir, at)
	// 옆에 임시 파일로 먼저 쓰고 옮긴다. 인코딩이 실패해도 반쪽짜리 PNG 가
	// 다운로드 폴더에 남지 않는다.
	tmp, err := os.CreateTemp(dir, ".camera-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

func photoDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

// uniquePath 는 같은 초에 두 번 찍어도 먼저 찍은 것을 덮지 않게 한다.
func uniquePath(dir string, at time.Time) string {
	base := "camera-" + at.Format("20060102-150405")
	path := filepath.Join(dir, base+".png")
	for i := 2; i < 100; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%d.png", base, i))
	}
	return path
}
