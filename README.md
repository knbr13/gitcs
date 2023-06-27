# local-git-contributions-visualizer

The `local-git-contributions-visualizer` is a command-line tool written in Go that enables developers to scan their local Git repositories and generate a visual contributions graph. This tool is particularly useful for developers who work with multiple Git services, such as GitHub and GitLab. It allows them to visualize their contributions across both platforms, even in offline or disconnected environments.

## Screenshots

![git-local-contributions-visualizer](https://raw.githubusercontent.com/abdullah-alaadine/local-git-contributions-visualizer/main/assets/screenshot.png)

## Features

- Scan local Git repositories and generate a contributions graph.
- Visualize contributions from Github and Gitlab services.
- Works offline, making it convenient for use in remote or disconnected environments.

## Development

1- Download and install `tdm-gcc` compiler to run binaries of `fyne/v2` package using the following URL:

```bash
  https://jmeubank.github.io/tdm-gcc/articles/2021-05/10.3.0-release
```

2- To clone the `local-git-contributions-visualizer` repository, copy and paste the following command:

```bash
  git clone https://github.com/abdullah-alaadine/local-git-contributions-visualizer.git
```

3- To run the tool, execute the following command:

```bash
  go run .
```

4- To run the GUI version of the tool, execute the following command:

```bash
  go run main.go --gui
```

5- To build the tool, run the following command:

```bash
  go build
```
## Installation

To install the local-git-contributions-visualizer, ensure that you have Go installed on your machine. Then, execute the following command:

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

## Hint

You can use the following command: Perhaps you want to check the email used for git on your machine.

```bash
git config --global user.email
```

## License

This project is licensed under the [MIT License](https://github.com/abdullah-alaadine/local-git-contributions-visualizer/blob/main/LICENSE). See the [LICENSE](https://github.com/abdullah-alaadine/local-git-contributions-visualizer/blob/main/LICENSE) file for details.

