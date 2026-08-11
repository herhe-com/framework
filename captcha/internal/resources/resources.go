package resources

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	assettiles "github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/codec"
	"github.com/wenlng/go-captcha/v2/slide"
)

// Backgrounds loads configured background images or embedded defaults.
func Backgrounds(root, dir string) ([]image.Image, error) {
	if dir == "" {
		return imagesv2.GetImages()
	}

	dir = filepath.Join(root, strings.Trim(dir, "/"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			files = append(files, entry.Name())
		}
	}
	if len(files) == 0 {
		return imagesv2.GetImages()
	}

	images := make([]image.Image, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return nil, err
		}

		var img image.Image
		switch strings.ToLower(filepath.Ext(file)) {
		case ".png":
			img, err = codec.DecodeByteToPng(data)
		case ".jpg", ".jpeg":
			img, err = codec.DecodeByteToJpeg(data)
		}
		if err == nil && img != nil {
			images = append(images, img)
		}
	}

	return images, nil
}

// Font loads a configured TTF font or the embedded default.
func Font(root, dir string) (*truetype.Font, error) {
	if dir == "" {
		return fzshengsksjw.GetFont()
	}

	dir = filepath.Join(root, strings.Trim(dir, "/"))
	entries, _ := os.ReadDir(dir)
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.EqualFold(filepath.Ext(entry.Name()), ".ttf") {
			files = append(files, entry.Name())
		}
	}
	if len(files) == 0 {
		return fzshengsksjw.GetFont()
	}

	data, err := os.ReadFile(filepath.Join(dir, files[rand.Intn(len(files))]))
	if err != nil {
		return nil, err
	}

	return freetype.ParseFont(data)
}

// Tiles loads configured slide tile resources or embedded defaults.
func Tiles(root, dir string) ([]*slide.GraphImage, error) {
	if dir != "" {
		if resources, err := loadTiles(filepath.Join(root, strings.Trim(dir, "/"))); err == nil && len(resources) > 0 {
			return resources, nil
		}
	}

	resources, err := assettiles.GetTiles()
	if err != nil {
		return nil, err
	}

	result := make([]*slide.GraphImage, 0, len(resources))
	for _, resource := range resources {
		result = append(result, &slide.GraphImage{
			OverlayImage: resource.OverlayImage,
			ShadowImage:  resource.ShadowImage,
			MaskImage:    resource.MaskImage,
		})
	}

	return result, nil
}

func loadTiles(dir string) ([]*slide.GraphImage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	result := make([]*slide.GraphImage, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		base := filepath.Join(dir, entry.Name())
		overlay, err := decodePNG(filepath.Join(base, "tile.png"))
		if err != nil {
			continue
		}
		shadow, err := decodePNG(filepath.Join(base, "tile-shadow.png"))
		if err != nil {
			continue
		}
		mask, err := decodePNG(filepath.Join(base, "tile-mask.png"))
		if err != nil {
			continue
		}

		result = append(result, &slide.GraphImage{
			OverlayImage: overlay,
			ShadowImage:  shadow,
			MaskImage:    mask,
		})
	}

	return result, nil
}

func decodePNG(file string) (image.Image, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return codec.DecodeByteToPng(data)
}
