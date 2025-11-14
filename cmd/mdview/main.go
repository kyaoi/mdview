package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/kyaoi/mdview/internal/app"
)

func main() {
	var tagMode bool
	flag.BoolVar(&tagMode, "t", false, "フロントマターの tags を表示して選択します")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] <path-to-markdown-or-directory>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	target := filepath.Clean(flag.Arg(0))
	if tagMode {
		if err := runTagSelection(target); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := app.Run(target); err != nil {
		log.Fatal(err)
	}
}

func runTagSelection(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	var index tagIndex
	if info.IsDir() {
		index, err = buildDirectoryTagIndex(path)
	} else {
		index, err = buildFileTagIndex(path)
	}
	if err != nil {
		return err
	}
	if index.isEmpty() {
		fmt.Println("指定されたパスからフロントマターの tags は見つかりませんでした。")
		return nil
	}

	printTagMenu(index)
	selection, confirmed, err := promptTagSelection(len(index.tags))
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("タグ選択をキャンセルしました。")
		return nil
	}

	tag := index.tags[selection]
	files := index.filesByTag[tag]
	fmt.Printf("タグ \"%s\" を含むファイル:\n", tag)
	for _, file := range files {
		fmt.Printf("  - %s\n", file)
	}
	return nil
}

func readFrontMatterTags(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	metadata := make(map[string]interface{})
	if _, err := frontmatter.Parse(file, &metadata); err != nil {
		return nil, err
	}

	value, ok := metadata["tags"]
	if !ok {
		return nil, nil
	}

	return normalizeTags(value), nil
}

func normalizeTags(value interface{}) []string {
	var raw []string
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				raw = append(raw, s)
			}
		}
	case []string:
		raw = append(raw, v...)
	case string:
		parts := strings.Split(v, ",")
		for _, part := range parts {
			raw = append(raw, part)
		}
	}

	seen := make(map[string]struct{})
	var tags []string
	for _, tag := range raw {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		tags = append(tags, trimmed)
	}
	return tags
}

func promptTagSelection(limit int) (int, bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("番号を入力してください (0でキャンセル): ")
		input, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, false, err
		}

		value := strings.TrimSpace(input)
		if value == "" {
			if errors.Is(err, io.EOF) {
				return 0, false, nil
			}
			fmt.Println("入力が空です。番号を入力してください。")
			continue
		}

		choice, convErr := strconv.Atoi(value)
		if convErr != nil {
			fmt.Println("数字を入力してください。")
			if errors.Is(err, io.EOF) {
				return 0, false, convErr
			}
			continue
		}

		if choice == 0 {
			return 0, false, nil
		}
		if choice < 0 || choice > limit {
			fmt.Println("指定した番号は無効です。")
			if errors.Is(err, io.EOF) {
				return 0, false, fmt.Errorf("無効な番号: %d", choice)
			}
			continue
		}

		return choice - 1, true, nil
	}
}

type tagIndex struct {
	tags       []string
	filesByTag map[string][]string
}

func (ti *tagIndex) add(tag, file string) {
	if ti.filesByTag == nil {
		ti.filesByTag = make(map[string][]string)
	}
	entries := ti.filesByTag[tag]
	for _, existing := range entries {
		if existing == file {
			return
		}
	}
	ti.filesByTag[tag] = append(entries, file)
}

func (ti *tagIndex) finalize() {
	if len(ti.filesByTag) == 0 {
		return
	}
	for tag, files := range ti.filesByTag {
		sort.Strings(files)
		ti.filesByTag[tag] = files
	}
	tags := make([]string, 0, len(ti.filesByTag))
	for tag := range ti.filesByTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	ti.tags = tags
}

func (ti tagIndex) isEmpty() bool {
	return len(ti.tags) == 0
}

func printTagMenu(index tagIndex) {
	fmt.Println("検出されたタグ:")
	for i, tag := range index.tags {
		fmt.Printf("  %d) %s (%d件)\n", i+1, tag, len(index.filesByTag[tag]))
	}
	fmt.Println("  0) キャンセル")
}

func buildFileTagIndex(path string) (tagIndex, error) {
	tags, err := readFrontMatterTags(path)
	if err != nil {
		return tagIndex{}, err
	}
	if len(tags) == 0 {
		return tagIndex{}, nil
	}
	displayPath := path
	if abs, err := filepath.Abs(path); err == nil {
		if wd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(wd, abs); err == nil {
				displayPath = rel
			}
		}
	}
	var index tagIndex
	for _, tag := range tags {
		index.add(tag, displayPath)
	}
	index.finalize()
	return index, nil
}

func buildDirectoryTagIndex(root string) (tagIndex, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return tagIndex{}, err
	}
	var index tagIndex
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != absRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(d.Name()) {
			return nil
		}
		tags, err := readFrontMatterTags(path)
		if err != nil {
			return err
		}
		if len(tags) == 0 {
			return nil
		}
		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			relPath = path
		}
		for _, tag := range tags {
			index.add(tag, filepath.ToSlash(relPath))
		}
		return nil
	})
	if err != nil {
		return tagIndex{}, err
	}
	index.finalize()
	return index, nil
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", ".hg", ".svn", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func isMarkdown(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}
