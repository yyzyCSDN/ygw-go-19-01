package multipart

import (
	"context"
	"fmt"
)

type Inspector interface {
	Inspect(context.Context, Part) error
}
type BasicInspector struct{ MaxPartBytes int }

func (i BasicInspector) Inspect(ctx context.Context, part Part) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if i.MaxPartBytes > 0 && len(part.Data) > i.MaxPartBytes {
		return fmt.Errorf("part %d exceeds inspection limit", part.Number)
	}
	return nil
}
