# LocalSend Switch

A lightweight utility to help LocalSend's device discovery in VLAN-segmented local area networks.  

## Overview

### Problem Illustration

![Issue Illustration](pics/issue_illustration.drawio.png)  
> Figure 1: Illustration of the problem. 可以看到 VLAN 0 中的 LocalSend 客户端无法成功发现 VLAN 2 中的 LocalSend 客户端。  

LocalSend 采用 UDP 组播来发现局域网中其他 LocalSend 客户端的存在。然而，像校园网这种大型局域网，通常为了管理和减小广播域规模等目的，会将网络划分为多个 VLAN（虚拟局域网），即使是现实中距离很近的两个设备，也有可能在不同的 VLAN 中。  

* 比如我连接到校园网 WiFi 的电脑和连接有线校园网的实验室打印机电脑，虽然在同一间屋子，但就是处于不同网段的网络中。

不同 VLAN 之间的数据转发依赖于第三层路由设备来实现，不幸的是，LocalSend 向 `224.0.0.x` 组播地址及应用端口发送的 UDP 报文段是**不会被三层设备转发**的，而且其 TTL 值为 `1`，Wireshark 抓包如下：  

![Wireshark Capture](pics/wireshark_captured.png)  
> Figure 2: Wireshark 抓包显示 LocalSend 发送的组播 UDP 报文段的 TTL 值为 1。  

因此就有了明明两台设备近在咫尺，但是却没法互相发现对方 LocalSend 客户端的尴尬局面 ㄟ( ▔, ▔ )ㄏ。  

更难受的是，这些设备甚至采用的是动态 IP，可能会发生变动，就算我在 LocalSend 中手动添加了对方的 IP 地址，过一段时间后对方分配的 IP 变了就又全部木大了...   

### Solution

尽管多播被 VLAN 隔离了，但是咱发现办公区校园网在三层配置上是会转发单播包的，我可以通过单播和不同的 VLAN 中的主机进行通信。  

一个 LocalSend 客户端在尝试发现局域网内其他客户端时，会发送组播 UDP 包来声明自己的存在，其他客户端收到组播包后会通过**单播的 HTTP 请求**来在这个客户端上进行注册。  

* 详见 [LocalSend Protocol - Discovery](https://github.com/localsend/protocol/blob/main/README.md#3-discovery)  



## CLI Usage

```bash
./localsend-switch-windows-amd64.exe -h # Show help message
```

| Flag | Description |
|------|-------------|
| `--help` | Show help message |
| `--debug` | Enable debug logging |

| Option | Environment Variable | Description | Default Value |
|--------|----------------------|-------------|---------------|
| `--autostart ` | × | Set autostart on user login, can be `enable` or `disable`. <br><br> * Currently only support *Windows* |  |
| `--client-alive-check-interval` | `LOCALSEND_SWITCH_CLIENT_ALIVE_CHECK_INTERVAL` | Interval (in seconds) to check if local LocalSend client is still alive. | `10` |
| `--client-broadcast-interval` | `LOCALSEND_SWITCH_CLIENT_BROADCAST_INTERVAL` | Interval (in seconds) to broadcast presence of local LocalSend client to peer switches. | `10` |
| `--log-file` | `LOCALSEND_SWITCH_LOG_FILE_PATH` | Path to log file. Can be relative or absolute. | `"localsend-switch-logs/latest.log"` |
| `--log-file-max-size` | `LOCALSEND_SWITCH_LOG_FILE_MAX_SIZE` | Max size (in Bytes) of log file before rotation. | `5242880` (5 MiB) | 
| `--log-file-max-historical` | `LOCALSEND_SWITCH_LOG_FILE_MAX_HISTORICAL` | Max number of historical (rotated) log files to keep. | `5` |
| `--ls-addr` | `LOCALSEND_MULTICAST_ADDR` | LocalSend multicast address. | `"224.0.0.167"` |
| `--ls-port` | `LOCALSEND_SERVER_PORT` | LocalSend HTTP server (and multicast) port. | `53317` |
| `--peer-addr` | `LOCALSEND_SWITCH_PEER_ADDR` | IP Address of peer switch node. |  |
| `--peer-connect-max-retries` | `LOCALSEND_SWITCH_PEER_CONNECT_MAX_RETRIES` | Max retries to connect to peer switch before giving up. <br><br> * Set to a **negative** number for unlimited retries. | `10` |
| `--peer-port` | `LOCALSEND_SWITCH_PEER_PORT` | Port of peer switch node. | (Default to `--serv-port`) |
| `--secret-key` | `LOCALSEND_SWITCH_SECRET_KEY` | Secret key for secure communication with peer switch nodes. |  |
| `--serv-port` | `LOCALSEND_SWITCH_SERV_PORT` | Port to listen for incoming TCP connections from peer switch nodes. |  |
| `--work-dir` | `LOCALSEND_SWITCH_WORK_DIR` | Working directory of the process. | (Default to the [executable's directory](#working-directory)) |

## Configure via Environment Variables

(Linux example)

## Runtime Details

### 交换与注册机制

### 通信安全性

### Log Files

Log files are rotated according to the configuration. By default, the log file path is `localsend-switch-logs/latest.log`. After rotation, the log files are also stored **in the same directory**, with filename pattern `<log_name>_rotated.<number>.log`, for example:

```bash
localsend-switch-logs/
├── latest.log
├── latest_rotated.1.log
├── latest_rotated.2.log
├── latest_rotated.3.log
├── latest_rotated.4.log
└── latest_rotated.5.log
```

Here, `latest.log` is the current log file, `latest_rotated.1.log` is the most recently rotated log file, and `latest_rotated.5.log` is the oldest log file currently retained (`--log-file-max-historical=5`).   

### Working Directory

The working directory will default to the **executable's directory**.   

* You can specify relative paths for log files, for example:  

    ```bash
    ./localsend-switch-linux-amd64 --log-file=localsend-switch-logs/latest.log
    ```

    and the log file will be definitely created here:  

    ```bash
    somewhere/
    ├── localsend-switch-logs
    │   └── latest.log # <- here
    └── localsend-switch-linux-amd64
    ```


* This is especially useful when `--autostart` is **enabled**, as the program will be started by the system under a different working directory (usually the system directory).  
* You can also specify a custom working directory using the `--work-dir` command-line argument or the `LOCALSEND_SWITCH_WORK_DIR` environment variable.  


## Examples

## Build

0. Generate the protobuf code:

    ```bash
    go generate ./...
    ```

    It has been already generated in the repository, so you can skip this step.  

1. Install `protoc` and `protoc-gen-go`, refer to [the official guide](https://protobuf.dev/getting-started/gotutorial/#compiling-protocol-buffers) for installation instructions.  

2. Build the project: 

    ```bash
    go build -o localsend-switch
    # Cross compilation
    GOOS=linux GOARCH=amd64 go build -o compiled/localsend-switch-linux-amd64
    GOOS=windows GOARCH=amd64 go build -o compiled/localsend-switch-windows-amd64.exe
    GOOS=darwin GOARCH=amd64 go build -o compiled/localsend-switch-macos-amd64
    # Make it start without a cmd window (run silently) on Windows
    GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o compiled/localsend-switch-windows-amd64-silent.exe
    ```

## Related Work

* [LocalSend](https://github.com/localsend/localsend)  
* [LocalSend Protocol](https://github.com/localsend/protocol)  