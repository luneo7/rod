package utils_test

import (
	"errors"

	"github.com/go-rod/rod/lib/utils"
)

func (t T) Promise() {
	p := utils.NewPromise()

	go func() {
		p.Done("ok", errors.New("err"))
	}()

	<-p.Wait()
	<-p.Wait()

	p.Done(nil, nil)

	val, err := p.Result()
	t.Eq(val, "ok")
	t.Eq(err.Error(), "err")
}
