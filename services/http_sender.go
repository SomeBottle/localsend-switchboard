package services

// 基于 HTTP 协议的数据发送模块

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
)

// setUpHTTPSender 启动 HTTP 请求发送器
//
// sendReqs: 要发送的 HTTP 请求，通道
// sigCtx: 中断信号上下文
func setUpHTTPSender(sendReqs <-chan *entities.HTTPJsonPostRequest, sigCtx context.Context) {
	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Transport: &http.Transport{
			// 跳过证书验证
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: constants.HTTPRequestTimeout * time.Second,
	}
	for {
		select {
		case <-sigCtx.Done():
			// 收到退出信号
			return
		case req, ok := <-sendReqs:
			if !ok {
				// 通道关闭，退出
				return
			}
			postRequest, err := http.NewRequest("POST", req.URL, bytes.NewReader(req.JsonBody))
			if err != nil {
				fmt.Println("Failed to create HTTP request:", err)
				continue
			}
			// 发送的是 JSON 数据
			postRequest.Header.Set("Content-Type", "application/json")
			response, err := httpClient.Do(postRequest)
			if err != nil {
				fmt.Println("Failed to send HTTP request:", err)
				continue
			}
			if response.StatusCode != http.StatusOK {
				fmt.Println("Received non-OK HTTP response:", response.Status)
			}else{
				fmt.Println("[DEBUG] Successfully sent HTTP request to", req.URL)
			}
			if req.RespChan != nil {
				// 如果有响应通道就读取响应体并发送回去
				respBody, err := io.ReadAll(io.LimitReader(response.Body, constants.HTTPResponseBodyMaxSize))
				_ = response.Body.Close()
				if err != nil {
					fmt.Println("Failed to read HTTP response body:", err)
					continue
				}
				httpResp:= &entities.HTTPResponse{
					StatusCode: response.StatusCode,
					Body:       respBody,
				}
				// 发送响应体回去
				select {
				case req.RespChan <- httpResp:
				case <-sigCtx.Done(): // 防止中断时仍在阻塞
					return
				}
			} else {
				// 响应体是一定要关闭的
				_ = response.Body.Close()
			}
		}
	}
}
