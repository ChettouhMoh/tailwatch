package notifier

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
)

type StdoutNotifier struct {
    writer io.Writer
}

func NewStdoutNotifier() *StdoutNotifier {
    return &StdoutNotifier{writer: os.Stdout}
}

func (n *StdoutNotifier) Notify(ctx context.Context, event Event) error {
    jsonData, err := json.MarshalIndent(event, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }
    
    _, err = fmt.Fprintf(n.writer, "Event: %s\n", string(jsonData))
    if err != nil {
        return fmt.Errorf("failed to write event: %w", err)
    }
    
    return nil
}
