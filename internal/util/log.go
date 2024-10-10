package util

import (
	"bytes"
	"github.com/Masterminds/sprig/v3"
	"log/slog"
	"strings"
	"sync"
	"text/template"
)

var emptyTemplate = template.Must(template.New("empty").Parse(""))

type Logging struct {
	Before         string `mapstructure:"before"`
	After          string `mapstructure:"after"`
	BeforeLevel    string `mapstructure:"beforeLevel"`
	AfterLevel     string `mapstructure:"afterLevel"`
	LogOnCall      int    `mapstructure:"logOnCall"`
	beforeTemplate *template.Template
	afterTemplate  *template.Template
	callCount      *Counter
	mutex          sync.Mutex
}

func (l *Logging) GetLogBeforeMsg(data any) string {
	if l.Before == "" {
		return l.Before
	}
	return renderTemplate(l.getBeforeTemplate(), data)
}

func (l *Logging) GetLogAfterMsg(data any) string {
	if l.After == "" {
		return l.After
	}
	return renderTemplate(l.getAfterTemplate(), data)
}

func (l *Logging) LogBefore(data any) {
	if l.Before != "" && l.shouldLog(false) {
		logOutput(l.BeforeLevel, renderTemplate(l.getBeforeTemplate(), data))
	}
}

func (l *Logging) LogAfter(data any) {
	if l.After != "" && l.shouldLog(true) {
		logOutput(l.AfterLevel, renderTemplate(l.getAfterTemplate(), data))
	}
}

func (l *Logging) shouldLog(isAfterCall bool) bool {
	if l.LogOnCall == 0 {
		return false
	} else if l.LogOnCall == 1 {
		return true
	}
	if isAfterCall && l.Before != "" {
		//counter already incremented and trigger for the request cycle
		return true
	}

	if l.callCount == nil {
		l.mutex.Lock()
		l.callCount = &Counter{
			Active:    true,
			TriggerOn: l.LogOnCall,
		}
		l.mutex.Unlock()
	}
	l.callCount.Increment()
	trigger := l.callCount.ShouldTrigger()
	if trigger {
		l.callCount.Reset()
	}
	return trigger
}

func (l *Logging) getBeforeTemplate() *template.Template {
	if l.beforeTemplate == nil {
		l.mutex.Lock()
		l.beforeTemplate = parseTemplate("before", l.Before)
		l.mutex.Unlock()
	}
	return l.beforeTemplate
}
func (l *Logging) getAfterTemplate() *template.Template {
	if l.afterTemplate == nil {
		l.mutex.Lock()
		l.afterTemplate = parseTemplate("after", l.After)
		l.mutex.Unlock()
	}
	return l.afterTemplate
}

func parseTemplate(tplName, tplString string) *template.Template {
	if tplString == "" {
		return emptyTemplate
	}
	tpl := template.New(tplName).Funcs(sprig.FuncMap())
	tpl.Delims("[[", "]]")
	parsed, err := tpl.Parse(tplString)
	if err != nil {
		slog.Error("failed to parse log template", "template", tplString, slog.Any("error", err))
		return emptyTemplate
	} else {
		return parsed
	}
}

func renderTemplate(tpl *template.Template, data any) string {
	var out bytes.Buffer
	err := tpl.Execute(&out, data)
	if err != nil {
		slog.Error("Error executing template", slog.Any("error", err))
		return ""
	}
	return strings.TrimSpace(out.String())
}

func logOutput(level, lines string) {
	level = strings.ToLower(level)
	for _, line := range strings.Split(lines, "\n") {
		switch level {
		case "debug":
			slog.Debug(line)
		case "warn":
			slog.Warn(line)
		case "error":
			slog.Error(line)
		default:
			slog.Info(line)
		}
	}
}
