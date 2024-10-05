package template

import (
	"github.com/valyala/fasttemplate"
	"os"
)

type Data interface {
	Render(t *fasttemplate.Template) string
}

type Template string

type DataMap map[string]interface{}

func (d DataMap) Render(t *fasttemplate.Template) string {
	return t.ExecuteString(d)
}

type DataFunc struct {
	TagFunc fasttemplate.TagFunc
}

func (d DataFunc) Render(t *fasttemplate.Template) string {
	return t.ExecuteFuncString(d.TagFunc)
}

const (
	startTag = "[["
	endTag   = "]]"
)

func Render(template Template, data Data) string {
	t := fasttemplate.New(string(template), startTag, endTag)
	return data.Render(t)
}

func RenderToFile(template Template, fileName string, data Data) error {
	t := fasttemplate.New(string(template), startTag, endTag)
	content := data.Render(t)
	return os.WriteFile(fileName, []byte(content), 0644)
}

func MustRenderToFile(template Template, fileName string, data Data) {
	err := RenderToFile(template, fileName, data)
	if err != nil {
		panic(err)
	}
}
