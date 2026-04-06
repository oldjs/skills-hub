package handlers

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const captchaChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var captchaPatterns = map[byte][]string{
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'A': {"00100", "01010", "10001", "10001", "11111", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C': {"01110", "10001", "10000", "10000", "10000", "10001", "01110"},
	'D': {"11100", "10010", "10001", "10001", "10001", "10010", "11100"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01110", "10001", "10000", "10111", "10001", "10001", "01110"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'J': {"00111", "00010", "00010", "00010", "10010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
}

func captchaRedisKey(token string) string {
	return "captcha:" + token
}

func generateCaptchaText(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = captchaChars[r.Intn(len(captchaChars))]
	}
	return string(b)
}

func drawPattern(img *image.RGBA, ch byte, offsetX, offsetY, scale int, col color.RGBA) {
	pattern, ok := captchaPatterns[ch]
	if !ok {
		return
	}

	for y, row := range pattern {
		for x, pixel := range row {
			if pixel != '1' {
				continue
			}
			rect := image.Rect(offsetX+x*scale, offsetY+y*scale, offsetX+(x+1)*scale, offsetY+(y+1)*scale)
			draw.Draw(img, rect, &image.Uniform{col}, image.Point{}, draw.Src)
		}
	}
}

func generateCaptchaImage(text string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 160, 52))
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	bg := color.RGBA{255, 248, 240, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	accent := []color.RGBA{{234, 88, 12, 255}, {249, 115, 22, 255}, {194, 65, 12, 255}, {15, 23, 42, 255}}
	for i := 0; i < 4; i++ {
		col := accent[r.Intn(len(accent))]
		col.A = 36
		drawLine(img, r.Intn(160), r.Intn(52), r.Intn(160), r.Intn(52), col)
	}
	for i := 0; i < 36; i++ {
		col := accent[r.Intn(len(accent))]
		col.A = 72
		img.SetRGBA(r.Intn(160), r.Intn(52), col)
	}

	for i, ch := range text {
		col := accent[r.Intn(len(accent))]
		drawPattern(img, byte(ch), 12+i*36+r.Intn(3), 11+r.Intn(4), 3, col)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func drawLine(img *image.RGBA, x1, y1, x2, y2 int, col color.RGBA) {
	dx := abs(x2 - x1)
	dy := -abs(y2 - y1)
	sx := -1
	if x1 < x2 {
		sx = 1
	}
	sy := -1
	if y1 < y2 {
		sy = 1
	}
	err := dx + dy

	for {
		if image.Pt(x1, y1).In(img.Bounds()) {
			img.SetRGBA(x1, y1, col)
		}
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x1 += sx
		}
		if e2 <= dx {
			err += dx
			y1 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func CaptchaHandler(w http.ResponseWriter, r *http.Request) {
	text := generateCaptchaText(4)
	imgData := generateCaptchaImage(text)

	token, err := generateToken()
	if err != nil {
		http.Error(w, "生成验证码失败", http.StatusInternalServerError)
		return
	}

	if err := rdb.Set(context.Background(), captchaRedisKey(token), text, 5*time.Minute).Err(); err != nil {
		http.Error(w, "生成验证码失败", http.StatusInternalServerError)
		return
	}

	setSecureCookie(w, "captcha_id", token, 300)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(imgData)
}

var captchaValidateScript = redis.NewScript(`
local key = KEYS[1]
local input = string.upper(ARGV[1])
local answer = redis.call('GET', key)
if not answer then
    return 0
end
redis.call('DEL', key)
if input == string.upper(answer) then
    return 1
end
return 0
`)

func validateCaptcha(r *http.Request, input string) bool {
	cookie, err := r.Cookie("captcha_id")
	if err != nil || cookie.Value == "" || input == "" {
		return false
	}

	result, err := captchaValidateScript.Run(context.Background(), rdb, []string{captchaRedisKey(cookie.Value)}, strings.TrimSpace(input)).Int()
	if err != nil {
		return false
	}

	return result == 1
}
