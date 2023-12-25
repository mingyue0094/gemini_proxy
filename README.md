# gemini_proxy
兼容openai 格式、stream 接口。


----


# 下载镜像：

docker pull mingyue0094/gemini_proxy:20231225203659

# 启动容器

```
docker run --name gemini -itd --restart always \
-p 8080:8080 -e TZ=Asia/Shanghai \
-e ALL_PROXY=socks5://192.168.20.25:3000 \
-e GEMINI_API_KEY=AI***********************************MM \
mingyue0094/gemini_proxy:20231225203659
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
