package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
	if info.IsDir() {
		return fmt.Errorf("-t フラグはファイルに対してのみ指定できます: %s はディレクトリです", path)
	}

	tags, err := readFrontMatterTags(path)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		fmt.Println("このファイルではフロントマターの tags が見つかりませんでした。")
		return nil
	}

	fmt.Printf("%s に含まれるタグ:\n", filepath.Base(path))
	for i, tag := range tags {
		fmt.Printf("  %d) %s\n", i+1, tag)
	}
	fmt.Println("  0) キャンセル")

	selection, confirmed, err := promptTagSelection(len(tags))
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("タグ選択をキャンセルしました。")
		return nil
	}

	fmt.Printf("選択されたタグ: %s\n", tags[selection])
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
