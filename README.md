# git-commits-visualizer

The `git-commits-visualizer` is a command-line tool written in Go that enables developers to scan their local Git repositories and generate a visual contributions graph. This tool is particularly useful for developers who work with multiple Git services, such as GitHub and GitLab. It allows them to visualize their contributions across both platforms, even in offline or disconnected environments.

## Screenshots

![git-commits-visualizer](https://raw.githubusercontent.com/abdullah-alaadine/git-commits-visualizer/main/assets/screenshot3.png)
![git-commits-visualizer](https://raw.githubusercontent.com/abdullah-alaadine/git-commits-visualizer/main/assets/screenshot4.png)

## Features

- Scan local Git repositories and generate a contributions graph.
- Visualize contributions from Github and Gitlab services.
- Works offline, making it convenient for use in remote or disconnected environments.

## Development

1- Clone the `git-commits-visualizer` repository, copy and paste the following command:

```bash
  git clone https://github.com/abdullah-alaadine/git-commits-visualizer.git # using HTTPS

  OR

  git clone git@github.com:abdullah-alaadine/git-commits-visualizer.git # using SSH

  OR

  gh repo clone abdullah-alaadine/git-commits-visualizer # using GitHub CLI
```

2- Build the tool:

```bash
  go build
```

3- Run the executable file:

```bash
  .\<executable_file_name>.exe # Windows OS
  ./<executable_file_name>     # Linux OS || Mac OS
```

## Installation

To install the git-commits-visualizer, ensure that you have Go installed on your machine. Then, execute the following command:

```bash
  go install github.com/abdullah-alaadine/git-commits-visualizer@latest
```

## Hint

You can use the following command: Perhaps you want to check the email used for git on your machine.

```bash
git config --global user.email
```

## Contributions

Contributions are welcome! If you would like to contribute to this project, please follow these steps:

1- Fork the repository.

2- Create a new branch for your feature or bug fix.

3- Make the necessary changes and commit them.

4- Push your changes to your fork.

5- Submit a pull request describing your changes.

## License

This project is licensed under the [MIT License](https://github.com/abdullah-alaadine/git-commits-visualizer/blob/main/LICENSE). See the [LICENSE](https://github.com/abdullah-alaadine/git-commits-visualizer/blob/main/LICENSE) file for details.
