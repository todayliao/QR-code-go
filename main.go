package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type Req struct {
	FileAddress string `json:"fileAddress"`
	Name        string `json:"name"`
	Tc          int    `json:"tc"`
}

const (
	FontPath     = "./font/hanyiyongzidingshenggaojianti.ttf"
	TemplatePath = "./img/zht.jpeg"
)

var (
	httpRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	})
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	successUrl := fmt.Sprintf("%s/success", GetGlobalConfig().Domain)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()

	var req Req
	if err = json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	log.Infof("request body :%+v", req)

	qrFileName := fmt.Sprintf("%d", time.Now().UnixMilli())
	//生成二维码
	options := make([]Option, 0)
	options = append(options, WithHalftoneSrcFile(fmt.Sprintf("%s/%s.png", "./static", req.FileAddress)))
	options = append(options, WithLogoWidth(BIG))
	options = append(options, WithName(qrFileName))
	options = append(options, WithPath(GetGlobalConfig().TmpPath))

	contentUrl := fmt.Sprintf("%s?name=%s&tc=%d&img=%s",
		successUrl, req.Name, req.Tc, fmt.Sprintf("%s.%s", qrFileName, DefaultFileType))
	qrCode, err := NewQuCodeGen(contentUrl, options...).GenQrCode()
	if err != nil {
		log.Errorf("gen qr code err")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof("resp qrcode:%s, contentUrl:%s", qrCode, contentUrl)

	resp, err := json.Marshal(map[string]interface{}{
		"code": 200,
		"data": fmt.Sprintf("%s%s/%s", GetGlobalConfig().Domain, GetGlobalConfig().TmpPath, qrCode),
	})
	if err != nil {
		log.Errorf("json marshal err")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(resp)
	return
}

func success(w http.ResponseWriter, r *http.Request) {
	tmpUrl := fmt.Sprintf("%s%s", GetGlobalConfig().Domain, GetGlobalConfig().TmpPath)

	// 解析查询参数
	query := r.URL.Query()
	// 获取其他表单字段
	name := query.Get("name")
	tc := cast.ToInt32(query.Get("tc")) / 1000
	sourceImg := query.Get("img")

	log.Printf("upload info name:%s, tc:%v, tmpUrl:%s", name, tc, tmpUrl)

	img := NewResImg(TemplatePath, []ResImgOption{
		WithFontPath(FontPath),
		WithFontSize(30),
		WithContentImg(ContentImg{
			ImagePath: fmt.Sprintf("%s/%s", tmpUrl, sourceImg),
			Width:     280,
			Height:    280,
			LineWidth: 2,
			Padding:   10,
			X:         367,
			Y:         410,
		}),
		WithContents(GetSuccessContent(name, tc)),
		WithDstPath(fmt.Sprintf(".%s/", GetGlobalConfig().TmpPath)),
	})

	_, fileName, err := img.Gen()
	if err != nil {
		log.Errorf("img gen err:%s", err)
		return
	}
	redirectUrl := fmt.Sprintf("%s/%s", tmpUrl, fileName)

	log.Infof("gen img fileName:%s , redirectUrl：%s", fileName, redirectUrl)
	http.Redirect(w, r, redirectUrl, http.StatusFound)
}

func runHttp() {
	tmpPath := fmt.Sprintf("%s/", GetGlobalConfig().TmpPath)

	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", GetGlobalConfig().Port))
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/qrcode/gen", uploadFileHandler)
	mux.HandleFunc("/success", success)
	mux.Handle("/static/", http.StripPrefix("/", http.FileServer(http.Dir("."))))
	mux.Handle(tmpPath, http.StripPrefix("/", http.FileServer(http.Dir("."))))
	_ = http.Serve(listen, mux)
}

func main() {
	InitConfig()

	log.Infof("starting server config: %+v", GetGlobalConfig())

	_ = os.Mkdir(fmt.Sprintf(".%s/", GetGlobalConfig().TmpPath), os.ModePerm)
	runHttp()
}
