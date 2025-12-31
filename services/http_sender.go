package services

// 基于 HTTP 协议的数据发送模块

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"
	"log/slog"

	"github.com/somebottle/localsend-switch/configs"
	"github.com/somebottle/localsend-switch/entities"
)

// setUpHTTPSender 启动 HTTP 请求发送器
//
// 请求失败 (比如超时) 时会向 sendReqs 发送 nil 作为响应
//
// sendReqs: 要发送的 HTTP 请求，通道
// sigCtx: 中断信号上下文
func setUpHTTPSender(sendReqs <-chan *entities.HTTPJsonRequest, sigCtx context.Context) {
	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Transport: &http.Transport{
			// 跳过证书验证
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: configs.HTTPRequestTimeout * time.Second,
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
			var request *http.Request
			var err error
			switch req.Method {
			case "POST":
				request, err = http.NewRequest("POST", req.URL, bytes.NewReader(req.JsonBody))
				if err != nil {
					slog.Error("Failed to create HTTP POST request", "error", err)
					continue
				}
				// 发送的是 JSON 数据
				request.Header.Set("Content-Type", "application/json")
			case "GET":
				request, err = http.NewRequest("GET", req.URL, nil)
				if err != nil {
					slog.Error("Failed to create HTTP GET request", "error", err)
					continue
				}
			default:
				slog.Warn("Unsupported HTTP method", "method", req.Method, "request", req)
				continue
			}
			response, err := httpClient.Do(request)
			if err != nil {
				slog.Debug("Failed to send HTTP request", "error", err)
				if req.RespChan != nil {
					// 响应 nil
					req.RespChan <- nil
				}
				continue
			}
			if response.StatusCode != http.StatusOK {
				slog.Debug("Received non-OK HTTP response", "status", response.Status)
			} else {
				slog.Debug("Successfully sent HTTP request", "url", req.URL)
			}
			if req.RespChan != nil {
				// 如果有响应通道就读取响应体并发送回去
				respBody, err := io.ReadAll(io.LimitReader(response.Body, configs.HTTPResponseBodyMaxSize))
				_ = response.Body.Close()
				if err != nil {
					slog.Error("Failed to read HTTP response body", "error", err)
					continue
				}
				httpResp := &entities.HTTPResponse{
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
