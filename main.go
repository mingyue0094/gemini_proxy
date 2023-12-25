package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"context"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	log     *logrus.Logger
	logFile *os.File

	client     *genai.Client
	clientInit sync.Once
	model      *genai.GenerativeModel
	ctx        context.Context
)

// Message 结构体用于解析messages中的消息
type Message_data struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// post 请求的json 数据
type Data_body struct {
	Messages []Message_data `json:"messages"`
	Model    string         `json:"model"`
}

func main() {
	log_init()
	defer logFile.Close()
	defer client.Close()

	http.HandleFunc("/v1/chat/completions", HandleGenerateRequest)
	port := ":8080"
	log.Printf("Server is listening on port %s...\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func HandleGenerateRequest(w http.ResponseWriter, r *http.Request) {
	// 设置允许所有跨域请求的头部
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	log.Debugln("--------\n", "收到请求:")

	// 打印请求方法
	log.Debugln("请求方法:", r.Method)

	// 打印请求头
	log.Debugln("请求头:")
	for key, values := range r.Header {
		for _, value := range values {
			log.Debugf("%s: %s\n", key, value)
		}
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "无法读取请求体", http.StatusInternalServerError)
		return
	}

	// 打印请求体
	log.Debugln("请求体:")
	log.Debugln(string(body))

	// 显示具体数据类型
	log.Println("请求体数据类型:", http.DetectContentType(body))

	// 处理预检
	if r.Method == "OPTIONS" {
		// 对于OPTIONS请求，可以在这里设置其他必要的响应头

		// 设置"Allow"头部，指明支持的HTTP方法
		w.Header().Set("Allow", "POST")

		// 返回状态码200 OK，因为这是预检请求的响应
		w.WriteHeader(http.StatusOK)

	}

	// 处理post
	fun_post_Request(w, body)

}

// post 请求
func fun_post_Request(w http.ResponseWriter, body []byte) {

	// 设置响应头，表明是一个流式响应
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Transfer-Encoding", "chunked")

	/**
	// 向客户端发送数据
	data := "123456abc"
	for _, char := range data {
		fmt.Fprintf(w, "%c", char)
		w.(http.Flusher).Flush() // 刷新缓冲区，将数据发送到客户端

		// 模拟处理延迟，实际中根据需要调整
		time.Sleep(500 * time.Millisecond)
	}
	*/

	Generatetext(w, body)

}

// 调用 谷歌 ai
func Generatetext(w http.ResponseWriter, body []byte) {

	log.Debug("body:", body)

	// 解析JSON字符串
	var data Data_body
	err2 := json.Unmarshal(body, &data)
	if err2 != nil {
		fmt.Println("解析JSON时出错:", err2)
		return
	}

	log.Debug(data)

	// 提取messages字段的所有内容
	var message_txt string
	for _, msg := range data.Messages {
		log.Debugf("Role: %s, Content: %s\n", msg.Role, msg.Content)
		message_txt = fmt.Sprintf("%s\nRole: %s, Content: %s\n", message_txt, msg.Role, msg.Content)
	}

	// // 打印请求体
	// log.Debugln("请求体:")
	// log.Debugln(string(body))
	google_ai(w, message_txt)

}

func google_ai(w http.ResponseWriter, Prompt string) {

	// Ensure client is initialized
	clientInit.Do(InitializeGenerativeClient)

	log.Debugln("对模型的提问", Prompt)
	iter := model.GenerateContentStream(ctx, genai.Text(Prompt))
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}

		// 获取 resp
		printResponse(w, resp)

	}

}

func printResponse(w http.ResponseWriter, resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				log.Debugln(part)
				stream_retrn(w, fmt.Sprintf("%s", part))
			}
		}
	}
	log.Debugln("-----------------")
}

// txt 文本，转流式 返回
func stream_retrn(w http.ResponseWriter, datatmp string) {
	datatmp = strings.Replace(datatmp, "\\n", "\n", -1)
	for _, char := range datatmp {
		fmt.Fprintf(w, "%c", char)
		w.(http.Flusher).Flush() // 刷新缓冲区，将数据发送到客户端

		// 模拟处理延迟，实际中根据需要调整
		//time.Sleep(500 * time.Millisecond)
	}
}

// InitializeGenerativeClient initializes the generative AI client once.
func InitializeGenerativeClient() {
	ctx = context.Background()
	var err error
	//client, err = genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	client, err = genai.NewClient(ctx, option.WithAPIKey("AIzaSyCrZ422BcTvpzRhYIsd5lbGozEzzbqIQVs"))

	if err != nil {
		log.Fatal(err)
	}
	// defer client.Close()  // 在主函数进行关闭

	model = client.GenerativeModel("gemini-pro")

}

// 日志
func log_init() {

	log = logrus.New()
	// 设置日志格式
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 构造日志文件的路径
	logFilePath := filepath.Join("./", ".debug")

	// 判断文件是否存在
	if _, err := os.Stat(logFilePath); err == nil {
		fmt.Println("调试模式")
		// 设置日志级别
		log.SetLevel(logrus.DebugLevel)

		// 打开文件以写入日志
		logFile, err := os.Create(".debug")
		if err != nil {
			log.Fatal("无法创建日志文件:", err)
		}
		// defer logFile.Close()

		// 设置日志输出到文件和控制台
		log.SetOutput(io.MultiWriter(logFile, os.Stdout))

		// 以下是示例日志记录
		//log.Info("这是一条示例日志信息")
		//log.Warn("这是一条示例警告信息")
		//log.Error("这是一条示例错误信息")

	} else {
		fmt.Println("静默模式")
		log.SetLevel(logrus.InfoLevel)

	}
}
