# LocalSend Switch

![LOGO](pics/localsend-switch-logo-small.png)  

A lightweight utility to help LocalSend's device discovery in VLAN-segmented local area networks.  

> 目前适配 LocalSend Protocol v2.1  

## Overview

### Problem Illustration

![Issue Illustration](pics/issue_illustration.drawio.png)  
> Figure 1: Illustration of the problem. 可以看到 VLAN 0 中的 LocalSend 客户端无法成功发现 VLAN 2 中的 LocalSend 客户端，反之亦然。  

LocalSend 客户端采用 UDP 组播来把自己的存在通告给局域网中其他客户端。然而，像校园网这种大型局域网，通常为了管理和减小广播域规模等目的，会将网络划分为多个 VLAN（虚拟局域网），即使是现实中距离很近的两个设备，也有可能在不同的 VLAN 中。  

* 比如我连接到校园网 WiFi 的电脑和连接有线校园网的实验室打印机电脑，虽然在同一间屋子，但就是处于不同网段的网络中。

不同 VLAN 之间的数据转发依赖于第三层路由设备来实现，不幸的是，LocalSend 向 `224.0.0.x` 组播地址及应用端口发送的 UDP 报文段是**不会被三层设备转发**的，而且其 TTL 值为 `1`，Wireshark 抓包如下：  

![Wireshark Capture](pics/wireshark_captured.png)  
> Figure 2: Wireshark 抓包显示 LocalSend 发送的组播 UDP 报文段的 TTL 值为 1。  

因此就有了明明两台设备近在咫尺，但是却没法互相发现对方 LocalSend 客户端的尴尬局面 ㄟ( ▔, ▔ )ㄏ。  

更难受的是，这些设备甚至采用的是动态 IP，可能会发生变动，就算我在 LocalSend 中手动添加了对方的 IP 地址，过一段时间后对方分配的 IP 变了就又全部木大了...   

### Solution

尽管多播被 VLAN 隔离了，但是咱发现办公区校园网在三层配置上是会转发单播包的，我可以通过单播和不同的 VLAN 中的主机进行通信。  

一个 LocalSend 客户端在尝试发现局域网内其他客户端时，会发送组播 UDP 包来声明自己的存在，其他客户端收到组播包后会通过**单播的 HTTP 请求**来在这个客户端上进行注册。因为单播可以跨 VLAN，所以这个注册操作是可以实现的，我可以替 LocalSend 客户端向局域网内的其他 LocalSend 客户端发送注册请求，从而实现跨 VLAN 的发现和注册。

* 详见 [LocalSend Protocol - Discovery](https://github.com/localsend/protocol/blob/main/README.md#3-discovery)  

从官方的协议文档可以看到 LocalSend 的通告包和注册请求的负载中都只有端口信息，没有源 IP 信息，客户端在处理到来的请求时实际上是**从网络层分组头部获取到 IP 地址**的，因此这个请求必须从 LocalSend 客户端所处的主机上发出。为了实现这点，我可以在每台有 LocalSend 的主机上都额外运行一个工具进程来代发注册请求。  

关键的问题来了，这些工具进程怎么知道局域网内其他 LocalSend 客户端的存在呢？其实我可以借助单播传输来实现这些工具进程之间的通信，从而让它们**互相交换**各自了解的 LocalSend 客户端信息。  

为了解决动态 IP 的问题，我可以把其中一个或多个工具进程作为交换节点**部署在拥有静态 IP 的服务器**上（内网和外网的均可），然后让其他工具进程连接到这些交换节点，当交换过程收敛时，这些工具进程就能互相了解对方所处主机上的 LocalSend 客户端信息了。  

这一套实现下来，LocalSend Switch 这个工具就诞生辣！٩(>௰<)و  

![Switch Strategy Illustration](pics/switch_strategy_illustration.drawio.png)   
> Figure 3: LocalSend Switch 的工作原理示意图。实线表示的是单播分组的传播路径，虚线表示的是 TCP 逻辑连接；虚线上的箭头对应数据在逻辑上的传播方向。LocalSend 客户端和 Switch 进程的旁边标记了连接端口，只有 VLAN 1 中的 Switch 进程监听了服务端口 `7761`，其余两个 Switch 进程的均为 OS 分配的临时端口；LocalSend 客户端默认服务端口是 `53317`。  

Fig.3 为 LocalSend Switch 的工作原理示意图，展示了单次的客户端信息传播以及注册请求代发的过程。图中，首先 `10.84.0.0/15` 网段中 `10.84.123.223` 这台主机上的 LocalSend 客户端发送了组播包，通告自己的存在，被同一台机器上的 LocalSend Switch 捕获到，Switch 进程随后将该通告信息通过单播发送 (图中标记为 `CLIENT ANNOUNCE`，传播路径为蓝色) 给它所连接的所有 Switch 节点 (图中只有 `192.168.232.47:7761` 这一个)。

> 发送的数据中封装了 **LocalSend 客户端的 IP 和端口**，无论被转发多少次，这部分数据都不会变，指向**最初发出**这条通告信息的 LocalSend 客户端。    

`47` 主机上 Switch 节点接收到通告的客户端信息后，会将该信息转发至它所连接的**其他** Switch 节点（图中只有 `10.94.23.114:52341`），图中标记为 `FORWARD ANNOUNCE`，传播路径为紫色。因为这台主机上没有 LocalSend 客户端，所以不会有注册请求的代发操作。  

`114` 主机上的 Switch 节点接收到通告信息后，会将该信息发送给它所连接的其他所有 Switch 节点（图中没有其他节点了）；因为这台主机上有 LocalSend 客户端，所以 Switch 节点随后会向通告信息中携带的 LocalSend 客户端地址 (图中为 `10.84.123.223:53317` ) 发送 HTTP(S) 注册请求（图中标记为 `REGISTER CLIENT`，传播路径为棕色），告知对方本地客户端的 IP 和地址 (图中为 `10.94.23.114:53317`)，完成注册请求的代发操作。注意这个注册请求是直接由 Switch 发送给 LocalSend 客户端的。  

实际上每个 Switch 节点都有这样的转发功能，你甚至可以在逻辑上串联或者组成树形、星型、网状、混合等拓扑结构。


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

你可以直接通过环境变量来配置 LocalSend Switch，只需将上表中的环境变量设置为对应的值，写入 `localsend-switch.env` 文件，并放在和可执行文件同目录下即可：  

```bash
somewhere/
    ├── localsend-switch.env # <- here
    └── localsend-switch-linux-amd64
```

示例 `localsend-switch.env` 文件内容：

```bash
LOCALSEND_SWITCH_SERV_PORT=7761
LOCALSEND_SWITCH_SECRET_KEY=el_psy_kongroo
```

## Runtime Details

### 本地客户端探测与主动广播

LocalSend Switch 会定期检查本地是否有 LocalSend 客户端在运行，默认间隔为 `10` 秒（可通过 `--client-alive-check-interval` 配置）。  

* 如果本地客户端发送了 UDP 组播包，Switch 会立即捕捉到并判定本地有客户端在运行。

一旦发现本地有 LocalSend 客户端在运行，Switch 会每隔一段时间（默认 `10` 秒，可通过 `--client-broadcast-interval` 配置）向它所连接的所有 Switch 节点广播本地客户端的信息。

这样一来用户不需要手动点击 LocalSend 客户端的设备列表刷新按钮，过一段时间后也能自动发现局域网中的其他客户端。  

### 交换与注册机制

每一个 LocalSend Switch 都可能担当以下两个角色中的一个或多个：  

1. **信息交换节点**：① 监听 `--serv-port` 指定的端口，等待其他 Switch 节点的 TCP 连接请求，建立连接；② 接收所有 Switch 节点连接上发来的 LocalSend 客户端信息 (每条信息会标记其来源的连接)，存入缓冲区；③ 给所有 Switch 节点连接发送*缓冲区中的 LocalSend 客户端信息*，每一条信息都会发给**除其来源连接以外**的其他连接。
2. **客户端辅助节点**：① 通过 `--peer-addr` 和 `--peer-port` 的配置连接到另一个 Switch 节点；② 捕捉本地 LocalSend 客户端发出的 UDP 组播包，把包中的本地客户端信息送入缓冲区；③ 在收到其他 Switch 节点转发过来的 LocalSend 客户端信息时，**代替本地客户端向信息中指明的客户端地址发送 HTTP(S) 注册请求**。  

总的来说，*缓冲区中的 LocalSend 客户端信息*来自:  

1. 本地客户端探测。  
2. 其他 Switch 节点转发过来的客户端信息。  

为了避免交换过程中产生环路，防止每条 LocalSend 客户端信息在 Switch 网络中无限制地传播，每条信息都携带了:  

1. **TTL（存活时间）字段**：每经过一个 Switch 节点，TTL 减 `1`，当 TTL 减到 `0` 时，该信息将不再被转发。默认 TTL 为 `255`。  
2. **唯一 ID 字段**：每条信息都有一个唯一 ID，由 Switch 节点的临时随机标识以及消息的递增编号组成。每个 Switch 节点都会**避免重复把相同 ID 的客户端信息重复加入缓冲区**。  
    * 不过每个 ID 在缓存中也是有 TTL 的，默认是 `5` 分钟。  

### 通信安全性

Switch 节点间的数据传输在 TCP 连接上进行，默认情况下是**明文**的，其中主要是 LocalSend 客户端的主机的地址、设备型号等信息。  

尽管在校园网这种较为可信的局域网中不用担心遭到中间人攻击，而且传输的数据本身也没有那么敏感，但如果中间有的 Switch 节点在外网上，就还是有一定风险的，如中间人可以伪造 LocalSend 客户端信息，诱导其他 Switch 节点向恶意构造的内网客户端地址发送注册请求，从而造成拒绝服务攻击 (DoS)。  

因此建议用 `--secret-key` 配置一个**对称加密密钥**，Switch 节点会利用该密钥对传输的数据进行端侧 **AES 加密**，只有持有相同密钥的节点才能解密和处理这些信息，从而提高通信的安全性（这里不采用非对称加密，本项目的场景和复杂度不太用得上，这样简单易用就行）。

> 💡 另外为了防止接收到恶意构造的 LocalSend 客户端信息，限制每个 Switch 节点仅可向**私有 IP 地址**发送 HTTP(S) 注册请求；上述的每条消息有唯一 ID 也可以一定程度上防止重放攻击。

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

这里构造一个简单的星型拓扑结构，假设局域网有六台主机 A, B, C, D, E, F，其中 D 为服务器，有静态 IP 地址 `192.168.232.47`；其他 A, B, C, E, F 均为 PC 计算机，有 LocalSend 客户端。  

* 在 D 上运行 LocalSend Switch，监听端口 `7761`，作为中心交换节点，启用端侧加密：  

    ```bash
    ./localsend-switch-linux-amd64 --serv-port=7761 --secret-key=el_psy_kongroo
    ```

* 在 A, B, C, E, F 上运行 LocalSend Switch，连接到 D：  

    ```bash
    # Set --peer-connect-max-retries to -1 for unlimited retries in case the server D is temporarily unreachable
    ./localsend-switch-windows-amd64.exe --peer-addr 192.168.232.47 --peer-port 7761 --secret-key=el_psy_kongroo --peer-connect-max-retries -1
    ```



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