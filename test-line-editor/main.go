// Throwaway test for go-multiline-ny: run it in a real terminal and try each key.
package main

import (
      "context"
      "errors"
      "fmt"
      "io"
      "os"
      "strings"

      "github.com/hymkor/go-multiline-ny"
      "github.com/nyaosorg/go-readline-ny"
      "github.com/nyaosorg/go-readline-ny/keys"
      "github.com/nyaosorg/go-readline-ny/simplehistory"
)

func main() {
      var ed multiline.Editor

      // [agent] "> " on the first line, "  " on continuation lines
      ed.SetPrompt(func(w io.Writer, lnum int) (int, error) {
              if lnum == 0 {
                      return fmt.Fprint(w, "> ")
              }
              return fmt.Fprint(w, "  ")
      })

      // [agent] history: ↑/↓ move between lines first, then into history at the edges
      history := simplehistory.New()
      ed.SetHistory(history)
      ed.SetHistoryCycling(true)

      // [agent] the library's default is the reverse (Enter = new line, Ctrl+J = submit); swap to chat-style
      ed.BindKey(keys.CtrlM, readline.AnonymousCommand(ed.Submit))
      ed.BindKey(keys.CtrlJ, readline.AnonymousCommand(ed.NewLine))
      ed.BindKey(keys.Escape+"\r", readline.AnonymousCommand(ed.NewLine)) // Alt/Option+Enter

      fmt.Println("Enter: submit | Ctrl+J or Alt+Enter: new line | Ctrl+C: abort line | Ctrl+D (empty): quit")

      ctx := context.Background()
      for {
              lines, err := ed.Read(ctx)
              if errors.Is(err, readline.CtrlC) {
                      fmt.Println("[Ctrl+C]")
                      continue
              }
              if errors.Is(err, io.EOF) {
                      fmt.Println("[Ctrl+D] bye")
                      return
              }
              if err != nil {
                      fmt.Fprintln(os.Stderr, "error:", err)
                      return
              }
              text := strings.Join(lines, "\n")
              fmt.Printf("--- got %d line(s) ---\n%s\n---\n", len(lines), text)
              history.Add(text)
      }
}
