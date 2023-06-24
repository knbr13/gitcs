# local-git-contributions-visualizer

The `local-git-contributions-visualizer` is a command line tool written in Go. It allows developers to scan their local Git repositories and generate a visual contributions graph. This tool is particularly useful for developers who work with multiple Git services such as Github and Gitlab. It enables them to see their contributions across both platforms, even when there is no internet connection available.

## Screenshots

![git-local-contributions-visualizer](https://raw.githubusercontent.com/abdullah-alaadine/local-git-contributions-visualizer/main/assets/screenshot.png)

## Features

- Scan local Git repositories and generate a contributions graph
- Visualize contributions from Github and Gitlab services
- Works offline, making it convenient for use in remote or disconnected environments

## Installation

To install the `local-git-contributions-visualizer`, make sure you have Go installed on your machine. Then, run the following command:

```bash
  go install github.com/abdullah-alaadine/local-git-contributions-visualizer@latest
```

## Usage

1- Run the local-git-contributions-visualizer executable:

```bash
./local-git-contributions-visualizer

```

2- Enter your Git email address when prompted:

```bash
Enter your Git email address: your-email@example.com

```

3- Enter the folder path to scan for Git repositories:

```bash
Enter the folder path to scan for Git repositories: /path/to/repository
```

License

This project is licensed under the [MIT License](https://github.com/abdullah-alaadine/local-git-contributions-visualizer/blob/main/LICENSE). See the [LICENSE](https://github.com/abdullah-alaadine/local-git-contributions-visualizer/blob/main/LICENSE) file for details.
```
