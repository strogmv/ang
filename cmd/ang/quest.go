package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/strogmv/ang/compiler/gamepoc"
)

func runQuest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("использование: ang quest export|compare|validate|edge-id")
	}
	switch args[0] {
	case "export":
		flags := flag.NewFlagSet("quest export", flag.ContinueOnError)
		game := flags.String("game", ".", "корень игры")
		out := flags.String("out", "", "новая папка для экспорта")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *out == "" || flags.NArg() != 0 {
			return fmt.Errorf("использование: ang quest export --game <игра> --out <новая папка>")
		}
		root, err := filepath.Abs(*game)
		if err != nil {
			return err
		}
		files, err := gamepoc.ExportQuestView(root)
		if err != nil {
			return err
		}
		if err := os.Mkdir(*out, 0755); err != nil {
			return err
		}
		for name, body := range files {
			path := filepath.Join(*out, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(body), 0644); err != nil {
				return err
			}
		}
		return nil
	case "compare", "diff", "validate":
		if len(args) != 3 {
			return fmt.Errorf("использование: ang quest %s <база> <правки>", args[0])
		}
		base, err := gamepoc.ReadQuestView(args[1])
		if err != nil {
			return err
		}
		edited, err := gamepoc.ReadQuestView(args[2])
		if err != nil {
			return err
		}
		if args[0] == "validate" {
			if err := gamepoc.ValidateQuestView(edited, base); err != nil {
				return err
			}
			fmt.Println("Проверка файлов дизайнера пройдена.")
			return nil
		}
		diff, err := gamepoc.CompareQuestViews(base, edited)
		if err != nil {
			return err
		}
		if len(diff) == 0 {
			fmt.Println("Смысловых различий нет.")
			return nil
		}
		for _, line := range diff {
			fmt.Println("• " + line)
		}
		if args[0] == "diff" {
			return nil
		}
		return fmt.Errorf("смысловых различий: %d", len(diff))
	case "edge-id":
		if len(args) != 4 {
			return fmt.Errorf("использование: ang quest edge-id <откуда> <куда> <подпись>")
		}
		fmt.Println(gamepoc.QuestEdgeID(args[1], args[2], args[3]))
		return nil
	default:
		return fmt.Errorf("неизвестная команда quest %s", args[0])
	}
}
