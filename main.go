package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"sync"
)

var (
	log     *logrus.Logger
	logFile *os.File

	client     *http.Client
	clientInit sync.Once
	post_url   string //stream请求网址
	post_url_  string //普通请求网址
)

// Message 结构体用于解析messages中的消息
type Message_data struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}



// Function 结构体
type Fc struct{
	FX map[string]interface{}		`json:"function"`
}

// post 请求的json 数据
type Data_body struct {
	Messages []Message_data `json:"messages"`
	Model    string         `json:"model"`
	Temperature float64		`json:"temperature"`
	Top_p float64			`json:"top_p"`
	Tools []Fc				`json:"tools"`
}

// stream 返回相关
type Delta struct {
	Content string `json:"content"`
}

type Choice struct {
	Delta Delta `json:"delta"`
}

type MyStruct struct {
	Choices []Choice `json:"choices"`
}

func main() {
	log_init()
	defer logFile.Close()

	http.HandleFunc("/v1/chat/completions", HandleGenerateRequest)
	http.HandleFunc("/fyapp", HandlehcfyappRequest)
	port := ":8080"
	log.Printf("Server is listening on port %s...\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

// 提供hcfy.app 翻译服务。 只能用于几句话的，不能用来自动整页翻译。
func HandlehcfyappRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var jsonBody map[string]interface{}

	// 解析JSON数据
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&jsonBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 输出JSON数据到控制台
	log.Debugln("Received JSON:", jsonBody)

	// 要翻译的文本
	text := jsonBody["text"].(string)

	// abc翻译结果
	abc := gemini_text_rqeusts(text)

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responseJSON := map[string]interface{}{
		"text":   "你好，划词翻译",
		"from":   "英语",
		"to":     "中文",
		"result": []string{abc},
	}
	json.NewEncoder(w).Encode(responseJSON)
}

// gemini 普通请求
func gemini_text_rqeusts(prompt string) string {

	// 构造 payload
	payload := fmt.Sprintf(`{"contents":[{"parts":[{"text": "请把以下内容翻译为中文\n\n'''\n%s\n\n'''"}]}]}`, prompt)

	log.Debugln("提交内容：", prompt)
	// Ensure client is initialized
	clientInit.Do(InitializeGenerativeClient)

	req, err := http.NewRequest("POST", post_url_, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		log.Debugln("Error creating request:", err)

	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("请求错误:", err)
		return ""
	}
	defer resp.Body.Close()

	log.Debugln("状态码:", resp.Status)

	// 处理响应
	if resp.StatusCode == http.StatusOK {

		// 使用 Scanner 逐行读取响应内容
		log.Debugln("fyapp Request successful. Response:")
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			// 如果行中包含 "text"，则打印该行
			if strings.Contains(line, "text") {

				line = strings.Replace(line, `\"`, `"`, -1)
				log.Debugln(line)
				a := len(line)
				log.Debugln(a)

				b := strings.Index(line, "text")

				log.Debugln(b)

				// 删除开头和结尾多余的
				c := line[b+11 : a-4]

				// 去除多余符号
				c = strings.Replace(c, "\\n", "\n", -1)

				//index := strings.Index(line, "text:")
				log.Debugln(c)
				log.Debugln("-----------------")
				return c

			}
		}

		if err := scanner.Err(); err != nil {
			log.Debugln("Error reading response body:", err)
			return "err"
		}

		return "err"

	} else {
		fmt.Println("Request failed. Status code:", resp.StatusCode)
		return "err"
	}
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
	w.Header().Set("Content-Type", "text/event-stream")
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

func stringify(data interface{}, indent string) string {
	// json 接口数据，遍历提取 k v ，然后拼接为 string
	switch v := data.(type) {
	case map[string]interface{}:
		result := `{`
		for key, val := range v {
			if key != "url"{
				result += indent + "" + `"`+ key + `"`+": " +`"`+ stringify(val, indent+"") +`"`+ ","
			}
			
		}
		result = result[:len(result)-1]
		result += indent
		result += `}`
		return result
	case []interface{}:
		result := `[`
		for _, val := range v {
			result += indent + "" +`"`+ stringify(val, indent+"") + `"`+","
		}
		result = result[:len(result)-1]
		result += indent
		result += `]`
		return result
	default:
		return fmt.Sprintf("%v", v)
	}
}

func stringify2(data interface{})string{
	// 去除多余符号，使string 可以被json 格式化
	c:= stringify(data,"")
	c = strings.Replace(c, `}"`, `}`, -1)
	c = strings.Replace(c, `"{`, `{`, -1)
	c = strings.Replace(c, `]"`, `]`, -1)
	c = strings.Replace(c, `"[`, `[`, -1)
	return c
}

// 调用 谷歌 ai
func Generatetext(w http.ResponseWriter, body []byte) {

	//log.Debug("body:", body)

	// 解析JSON字符串
	var data Data_body
	err2 := json.Unmarshal(body, &data)
	if err2 != nil {
		fmt.Println("解析JSON时出错:", err2)
		return
	}

	log.Debug("==== 请求 data  start  ====")
	log.Debug(data)
	log.Debug("==== 请求 data  end    ====")
	length := len(data.Messages)
	log.Debug("==== data.Messages 数量    ====\n",length)

	log.Debug("==== data.tools 数量    ====\n",len(data.Tools))



	for _,tool := range data.Tools{
		log.Debug("==== 请求 tools    ====")
		log.Debug(tool.FX)
		a:=stringify2(tool.FX)
		log.Debug("==== ret: \n",a)

	}



	// // 打印提取的函数信息
	// fmt.Println("提取的函数信息:")
	// for _, tool := range data.Tools {
	// 	fmt.Println("函数名称:", tool.Fx.Description)

		// fmt.Println("函数描述:", tool.Description)
		// fmt.Println("函数参数:")
		// for paramName, paramDetails := range tool.Parameters {
		// 	fmt.Println("  参数名:", paramName)
		// 	paramType := paramDetails.(map[string]interface{})["type"].(string)
		// 	fmt.Println("  参数类型:", paramType)
		// 	paramDescription := paramDetails.(map[string]interface{})["description"].(string)
		// 	fmt.Println("  参数描述:", paramDescription)
		// }
		// fmt.Println("函数URL:", tool.URL)
		// fmt.Println()
	//}
	

	
	// 整理请求json
	if length == 1{
		// 只有一条
		for _, msg := range data.Messages{
			log.Debugf("Role: %s, Content: %s\n", msg.Role, msg.Content)
			if len(data.Tools) == 0{
				payload := fmt.Sprintf(`{"contents":[{"parts":[{"text": "%s"}]}]}`, msg.Content)
				google_ai(w, payload)
				break

			} else {
				tools :=""
				for _,tool := range data.Tools{
					tools += fmt.Sprintf(`%s,`,stringify2(tool.FX))
				}
				tools = tools[:len(tools)-1]
				payload := fmt.Sprintf(`{"contents":{"parts":{"text":"%s"},"role":"user"},"tools":[{"function_declarations":[%s]}]}`, msg.Content,tools)
				
				google_ai(w, payload)
				break


			}
				
			
			
		}
	} else {
		msg_txt := ""
		for _, msg := range data.Messages{
			log.Debugf("Role: %s, Content: %s\n", msg.Role, msg.Content)

			if msg.Role == "user"{
				msg_txt += fmt.Sprintf(`{"role":"user","parts":[{"text":"%s"}]},`,  msg.Content)
			} else if  msg.Role == "assistant" {
				msg_txt += fmt.Sprintf(`{"role":"model","parts":[{"text":"%s"}]},`, msg.Content)
			} 
		
		}
		msg_txt = msg_txt[:len(msg_txt)-1] //最后 ，  删除

		if len(data.Tools) == 0{
			payload := fmt.Sprintf(`{"contents":[%s]}`, msg_txt)
			google_ai(w, payload)
		} else {
			tools :=""
			for _,tool := range data.Tools{
				tools += fmt.Sprintf(`%s,`,stringify2(tool.FX))
			}
			tools = tools[:len(tools)-1]

			payload := fmt.Sprintf(`{"contents":[%s],"tools":[{"functionDeclarations":[%s]}]}`, msg_txt,tools)
			google_ai(w, payload)
		}

	}
	
	

	// // 提取messages字段的所有内容
	// var message_txt string
	// for _, msg := range data.Messages {
	// 	log.Debugf("Role: %s, Content: %s\n", msg.Role, msg.Content)
	// 	//message_txt = fmt.Sprintf("%s\n\n【%s】:%s\n", message_txt, msg.Role, msg.Content)
	// 	message_txt = fmt.Sprintf("%s\n\n%s\n", message_txt, msg.Content)

	// }

	// // // 打印请求体
	// // log.Debugln("请求体:")
	// // log.Debugln(string(body))
	// google_ai(w, message_txt)

}

func google_ai(w http.ResponseWriter, payload string) {
	// 打印请求体
	log.Debugln("请求体:")
	log.Debugln(payload)

	// Ensure client is initialized
	clientInit.Do(InitializeGenerativeClient)

	req, err := http.NewRequest("POST", post_url, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		log.Debugln("Error creating request:", err)

	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Transfer-Encoding", "chunked")

	resp, err2 := client.Do(req)
	if err2 != nil {
		log.Debugln("Error making request:", err2)
		return
	}

	// 获取 resp
	printResponse(w, resp)

}

func printResponse(w http.ResponseWriter, resp *http.Response) {
	// 使用 bufio.Scanner 逐行读取响应内容
	scanner := bufio.NewScanner(resp.Body)
	content :=""  // 返回内容
	for scanner.Scan() {
		line := scanner.Text()
		

		content += line


		// 如果行中包含 "text"，则打印该行
		if strings.Contains(line, "text") {
			
			a := len(line)
			log.Debugln(a)

			b := strings.Index(line, "text")

			log.Debugln(b)

			//删除开头和结尾多余的
			c := line[b+8 : a-1]

			// 去除多余符号
			c = strings.Replace(c, "\\n", "\n", -1)

			// /t 替换为4个空格
			c = strings.Replace(c, "\t", "    ", -1)

			// \" 替换为  "
			c = strings.Replace(c, `\"`, `"`, -1)

			// \' 替换为  '
			c = strings.Replace(c, `\'`, `'`, -1)

			//index := strings.Index(line, "text:")
			log.Debugln(c)
			stream_retrn(w, c)
			log.Debugln("-----------------")

		}
	}

	if err := scanner.Err(); err != nil {
		log.Debugln("Error reading response body:", err)
		return
	}

	// log.Debugln("==== 返回内容 start  ====")
	// log.Debugln(content)
	// log.Debugln("==== 返回内容 end  ====")

	// content = content_pare(content)
	// log.Debugln("==== 整理1  start  ====")
	// log.Debugln(content)
	// // realtime-weather____fetchCurrentWeather
	// // {"city": "上海"}

	// if strings.Contains(content, "functionCall"){
	// 	// 含有指定字符 
	// 	var data []map[string]interface{}
	// 	err := json.Unmarshal([]byte(content), &data)
	// 	if err != nil {
	// 		fmt.Println("Error:", err)
	// 		return
	// 	}
	
	// 	for _, candidate := range data {
	// 		candidates := candidate["candidates"].([]interface{})
	// 		for _, c := range candidates {
	// 			content := c.(map[string]interface{})["content"].(map[string]interface{})
	// 			parts := content["parts"].([]interface{})
	// 			for _, part := range parts {
	// 				functionCall := part.(map[string]interface{})["functionCall"].(map[string]interface{})
	// 				name := functionCall["name"].(string)
	// 				args_interface := functionCall["args"].(map[string]interface{})
	// 				fmt.Println("Function Name:", name)
	// 				fmt.Println("args:", stringify2(args_interface))
	// 			}
	// 		}
	// 	}

	// }



	// log.Debugln("==== 整理1  end  ====")

	//stream_retrn(w, "test")
	//回复结束
	jsondata := []byte("data:[DONE]\n\n")
	w.Write(jsondata)
	w.(http.Flusher).Flush() // 刷新缓冲区，将数据发送到客户端
}

// func content_pare(c string) string {
// 	// 去除多余符号
// 	c = strings.Replace(c, "\\n", "\n", -1)

// 	// /t 替换为4个空格
// 	c = strings.Replace(c, "\t", "    ", -1)

// 	// \" 替换为  "
// 	c = strings.Replace(c, `\"`, `"`, -1)

// 	// \' 替换为  '
// 	c = strings.Replace(c, `\'`, `'`, -1)
// 	return c	
// }


// txt 文本，转流式 返回
func stream_retrn(w http.ResponseWriter, datatmp string) {
	var jsondata []byte
	for _, char := range datatmp {
		//chunk: data:  {"id":"chatcmpl-ATdRBnZVzP4krnreNZ14OYLYqWyM5","object":"chat.completion.chunk","created":1703566659,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"？"},"finish_reason":null}]}
		//chunk: data:  {"id":"chatcmpl-ATdRBnZVzP4krnreNZ14OYLYqWyM5","object":"chat.completion.chunk","created":1703566659,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}
		//chunk: data:[DONE]

		/**
		普通
		结束
		完成
		**/

		// 组装结构
		myData := MyStruct{
			Choices: []Choice{
				{
					Delta: Delta{
						Content: string(char),
					},
				},
			},
		}
		// 将结构体转换为 JSON 字符串
		jsonString, err := json.Marshal(myData)
		if err != nil {
			fmt.Println("转换为 JSON 字符串失败:", err)
			return
		}

		jsondata = []byte(fmt.Sprintf("data: %s\n\n", jsonString))
		w.Write(jsondata)
		w.(http.Flusher).Flush() // 刷新缓冲区，将数据发送到客户端

	}
}

// InitializeGenerativeClient initializes the generative AI client once.
func InitializeGenerativeClient() {
	post_url = "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:streamGenerateContent?key=" + os.Getenv("GEMINI_API_KEY")
	post_url_ = "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=" + os.Getenv("GEMINI_API_KEY")

	if os.Getenv("ALL_PROXY") != "" {
		// 设置代理地址
		proxyURL, err := url.Parse(os.Getenv("ALL_PROXY"))
		if err != nil {
			fmt.Println("Error parsing proxy URL:", err)
			return
		}

		fmt.Println("使用 proxy:", proxyURL)

		// 创建一个自定义的 Transport
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}

		// 使用自定义的 Transport 创建一个 http.Client
		client = &http.Client{
			Transport: transport,
		}

	} else {
		log.Debugln("直接连接")
		client = &http.Client{}

	}

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
