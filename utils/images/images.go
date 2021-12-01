package images

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/RicheyJang/PaimengBot/utils"
	"github.com/RicheyJang/PaimengBot/utils/consts"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var defaultFont *truetype.Font

func init() {
	font, err := ParseFont(consts.DefaultTTFPath) // 加载默认字体文件
	if err != nil {                               // 加载失败，从默认字体目录中尝试遍历
		rd, _ := ioutil.ReadDir(consts.DefaultTTFDir)
		for _, file := range rd {
			if file.IsDir() {
				continue
			}
			font, err = ParseFont(utils.PathJoin(consts.DefaultTTFDir, file.Name()))
			if err == nil {
				log.Infof("成功加载字体文件：%v", file.Name())
				break
			}
		}
	}
	if err != nil || font == nil { // 全部失败
		log.Errorf("加载默认字体文件(%v)失败 err: %v", consts.DefaultTTFDir, err)
		return
	}
	defaultFont = font
	log.Infof("成功加载默认字体")
}

// ImageCtx 图片上下文
type ImageCtx struct {
	*gg.Context
}

// NewImageCtxWithBGPath 以背景图片路径创建带有背景图片的图片上下文
func NewImageCtxWithBGPath(w, h int, bgPath string, opacity float64) (*ImageCtx, error) {
	bg, err := gg.LoadImage(bgPath)
	if err != nil {
		return nil, err
	}
	return NewImageCtxWithBG(w, h, bg, opacity), nil
}

// NewImageCtxWithBG 创建带有背景图片的图片上下文，通过opacity设置不透明度
func NewImageCtxWithBG(w, h int, bg image.Image, opacity float64) *ImageCtx {
	if opacity > 0 && opacity < 1 {
		bg = AdjustOpacity(bg, opacity)
	}
	res := NewImageCtx(w, h)
	sx := float64(w) / float64(bg.Bounds().Size().X)
	sy := float64(h) / float64(bg.Bounds().Size().Y)
	// 记录原始状态
	res.Push()
	// 设置背景
	res.Scale(sx, sy)
	res.DrawImage(bg, 0, 0)
	// 恢复原始状态
	res.Pop()
	return res
}

// NewImageCtxWithBGRGBA255 以RGBA255形式创建纯色背景的图片上下文
func NewImageCtxWithBGRGBA255(w, h int, r, g, b, a int) *ImageCtx {
	res := NewImageCtx(w, h)
	// 记录原始状态
	res.Push()
	// 设置背景
	res.SetRGBA255(r, g, b, a)
	res.Clear()
	// 恢复原始状态
	res.Pop()
	return res
}

// NewImageCtx 创建全透明背景的图片上下文
func NewImageCtx(w, h int) *ImageCtx {
	dc := gg.NewContext(w, h)
	// 记录原始状态
	dc.Push()
	// 全透明
	dc.SetRGBA(1, 1, 1, 0)
	dc.Clear()
	// 恢复原始状态
	dc.Pop()
	return &ImageCtx{
		Context: dc,
	}
}

// SetFont 通过truetype.Font设置字体与字体大小
func (img *ImageCtx) SetFont(font *truetype.Font, size float64) error {
	if font == nil {
		return errors.New("point font is nil")
	}
	face := truetype.NewFace(font, &truetype.Options{
		Size: size,
	})
	img.SetFontFace(face)
	return nil
}

// UseDefaultFont 使用默认字体并设置字体大小
func (img *ImageCtx) UseDefaultFont(size float64) error {
	return img.SetFont(defaultFont, size)
}

var tempCountMutex sync.Mutex
var tempCount int64 = 0

// SaveTemp 以前缀prefix保存至临时图片文件夹
func (img *ImageCtx) SaveTemp(prefix string) (string, error) {
	// 获取临时序号
	tempCountMutex.Lock()
	tempCount = (tempCount + 1) % (viper.GetInt64("tmp.maxcount") + 1)
	fileName := fmt.Sprintf("%s_%v.png", prefix, tempCount)
	tempCountMutex.Unlock()

	// 尝试创建临时文件夹
	fullDir, err := utils.MakeDir(consts.TempImageDir)
	if err != nil {
		log.Errorf("创建临时目录或获取绝对路径失败 err：%v", err)
		return "", err
	}
	// 保存图片
	err = img.SavePNG(utils.PathJoin(consts.TempImageDir, fileName))
	if err != nil {
		log.Errorf("保存临时图片失败 err：%v", err)
		return "", err
	}
	return utils.PathJoin(fullDir, fileName), nil
}

// SaveTempDefault 以默认前缀(tempimg)保存至临时图片文件夹
func (img *ImageCtx) SaveTempDefault() (string, error) {
	return img.SaveTemp("tempimg")
}

func (img *ImageCtx) GenMessageBase64() (message.MessageSegment, error) {
	resultBuff := bytes.NewBuffer(nil) // 结果缓冲区
	// 新建Base64编码器（Base64结果写入结果缓冲区resultBuff）
	encoder := base64.NewEncoder(base64.StdEncoding, resultBuff)
	// 将图片PNG格式写入Base64编码器
	err := png.Encode(encoder, img.Image())
	if err != nil {
		_ = encoder.Close()
		return message.Text("图片生成失败"), err
	}
	// 结束Base64编码
	err = encoder.Close()
	if err != nil {
		return message.Text("图片Base64生成失败"), err
	}
	return message.Image("base64://" + resultBuff.String()), nil
}

var tmpAddressBuff = make(map[string]bool)

// 判断OneBot(消息收发端)是否在本地
func isOneBotLocal() (res bool) {
	addr := viper.GetString("server.address")
	defer func() {
		tmpAddressBuff[addr] = res
	}()
	if res, ok := tmpAddressBuff[addr]; ok { // 读取缓存
		return res
	}
	sub := strings.Split(addr, "//")
	if len(sub) < 2 {
		return false
	}
	if strings.HasPrefix(sub[1], "127") || strings.HasPrefix(sub[1], "local") {
		return true
	}
	return false
}

// GenMessageAuto 自动生成ZeroBot图片消息
func (img *ImageCtx) GenMessageAuto() (message.MessageSegment, error) {
	// 消息收发端不在本地
	if !isOneBotLocal() {
		return img.GenMessageBase64()
	}
	// 消息收发端位于本地
	file, err := img.SaveTempDefault()
	if err != nil { // 生成文件出错，尝试Base64
		return img.GenMessageBase64()
	}
	return message.Image("file:///" + file), nil
}