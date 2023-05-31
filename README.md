# local-git-contributions-visualizer

The `local-git-contributions-visualizer` is a command line tool written in Go. It allows developers to scan their local Git repositories and generate a visual contributions graph. This tool is particularly useful for developers who work with multiple Git services such as Github and Gitlab. It enables them to see their contributions across both platforms, even when there is no internet connection available.

## Screenshots

![git-local-contributions-visualizer](https://github.com/abdullah-alaadine/local-git-contributions-visualizer/assets/125296663/20937460-c8bc-41bb-90e4-cdb7fefd21f7)

## Features

- Scan local Git repositories and generate a contributions graph
- Visualize contributions from Github and Gitlab services
- Works offline, making it convenient for use in remote or disconnected environments

## Installation

To install the `local-git-contributions-visualizer`, make sure you have Go installed on your machine. Then, run the following command:

```bash
  go get github.com/abdullah-alaadine/local-git-contributions-visualizer
```

## Usage

The local-git-contributions-visualizer tool offers the following options:

1- add: Add a folder path to generate a graph based on the commits in that folder.

```bash
./local-git-contributions-visualizer -add /path/to/repository

```

2- email: Add your Github email address to track contributions associated with it.

```bash
./local-git-contributions-visualizer -email your-email@example.com

```

3- help: Display available options and usage instructions.

```bash
./local-git-contributions-visualizer -h
```

Certainly! Here's an updated version of the README file that includes instructions for downloading the built file:

markdown

# local-git-contributions-visualizer

![Tool Screenshot](screenshot.png)

The `local-git-contributions-visualizer` is a command line tool written in Go. It allows developers to scan their local Git repositories and generate a visual contributions graph. This tool is particularly useful for developers who work with multiple Git services such as Github and Gitlab. It enables them to see their contributions across both platforms, even when there is no internet connection available.

## Installation

To install the `local-git-contributions-visualizer`, you can choose one of the following methods:

### Method 1: Build from Source

1. Make sure you have Go installed on your machine.

2. Clone the repository:

```shell
git clone https://github.com/your-username/local-git-contributions-visualizer.git

    Change to the project directory:

shell

cd local-git-contributions-visualizer

    Build the tool:

shell

go build -o local-git-contributions-visualizer

Method 2: Download the Pre-built Binary

    Go to the Releases page on the repository.

    Download the appropriate pre-built binary for your operating system.

Usage

The local-git-contributions-visualizer tool offers the following options:

    add: Add a folder path to generate a graph based on the commits in that folder.

shell

./local-git-contributions-visualizer add /path/to/repository

    email: Add your Github email address to track contributions associated with it.

shell

./local-git-contributions-visualizer email your-email@example.com

    -h: Display available options and usage instructions.

shell

./local-git-contributions-visualizer -h

Examples

    Generate a contributions graph for a specific repository:

shell

./local-git-contributions-visualizer add /path/to/repository

    Add your Github email address:

shell

./local-git-contributions-visualizer email your-email@example.com

Requirements

    Go programming language

License

This project is licensed under the MIT License. See the LICENSE file for details.
```
