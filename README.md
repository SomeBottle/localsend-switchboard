# LocalSend Switch

A lightweight utility to help LocalSend's device discovery in VLAN-segmented local area networks.  

## Overview

![Issue Illustration](pics/issue_illustration.drawio.png)  

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
| `--serv-port` | `LOCALSEND_SWITCH_SERV_PORT` | Port to listen for incoming TCP connections from peer switch nodes. |  |
| `--work-dir` | `LOCALSEND_SWITCH_WORK_DIR` | Working directory of the process. | (Default to the [executable's directory](#working-directory)) |

## Runtime Details

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
    # Cross Compilation
    GOOS=linux GOARCH=amd64 go build -o compiled/localsend-switch-linux-amd64
    GOOS=windows GOARCH=amd64 go build -o compiled/localsend-switch-windows-amd64.exe
    GOOS=darwin GOARCH=amd64 go build -o compiled/localsend-switch-macos-amd64
    # Make it start without a cmd window (run silently) on Windows
    GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o compiled/localsend-switch-windows-amd64-silent.exe
    ```