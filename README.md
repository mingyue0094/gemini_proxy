# 下载镜像：

```
docker pull mingyue0094/gemini_proxy:20231228174134
```


# 启动容器

```
docker run --name gemini -itd --restart always \
-p 8080:8080 -e TZ=Asia/Shanghai \
-e ALL_PROXY=socks5://192.168.20.25:3000 \
-e GEMINI_API_KEY=AI***********************************MM \
mingyue0094/gemini_proxy:20231228174134
```

-   ALL_PROXY 可选
- GEMINI_API_KEY 必选。 gemini api key


# 设置debug
```
echo "" > .debug
docker cp .debug gemini:/.debug

docker restart gemini
 ```

看日志
```
docker logs -f gemini
```


# 关闭debug
删除容器，重开。


# openai_baseurl

```
http://ip:8080/v1
```

# chatgpt next
```
http://ip:8080
```

# hcfy.app 插件对应的， 自建翻译服务接口。
```
http://ip:8080/fyapp
```
 - 能单词，句子。整页翻译不动。
