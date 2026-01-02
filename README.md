# LocalSend Switch

A lightweight utility to help LocalSend's device discovery in VLAN-segmented local area networks.  

## Usage

## Working Directory

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


* This is especially useful when `autostart` is enabled, as the program will be started by the system with a different working directory (usually the system directory).  
* You can also specify a custom working directory using the `--work-dir` command-line argument or the `LOCALSEND_SWITCH_WORK_DIR` environment variable.  

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