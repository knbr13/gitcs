package main

import (
	"io/fs"
	"path/filepath"
	"strings"
)

type SourceFile struct {
	Path     string
	Language string
}

var languagesByExtension = map[string]string{
	".c":      "C",
	".cc":     "C++",
	".cpp":    "C++",
	".cs":     "C#",
	".css":    "CSS",
	".dart":   "Dart",
	".ex":     "Elixir",
	".exs":    "Elixir",
	".fs":     "F#",
	".fsx":    "F#",
	".go":     "Go",
	".h":      "C/C++",
	".hpp":    "C++",
	".html":   "HTML",
	".java":   "Java",
	".js":     "JavaScript",
	".jsx":    "JavaScript",
	".kt":     "Kotlin",
	".kts":    "Kotlin",
	".lua":    "Lua",
	".php":    "PHP",
	".py":     "Python",
	".r":      "R",
	".rb":     "Ruby",
	".rs":     "Rust",
	".scala":  "Scala",
	".sh":     "Shell",
	".sql":    "SQL",
	".svelte": "Svelte",
	".swift":  "Swift",
	".ts":     "TypeScript",
	".tsx":    "TypeScript",
	".vue":    "Vue",
}

func findSourceFiles(root string) ([]SourceFile, error) {
	var files []SourceFile

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			if _, excluded := excludedFolders[strings.ToLower(entry.Name())]; excluded {
				return filepath.SkipDir
			}
			return nil
		}

		extension := strings.ToLower(filepath.Ext(path))
		if language, supported := languagesByExtension[extension]; supported {
			files = append(files, SourceFile{
				Path:     path,
				Language: language,
			})
		}

		return nil
	})

	return files, err
}
