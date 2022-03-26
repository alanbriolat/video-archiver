package video_archiver

import (
	"strings"
	"text/template"
)

type DownloadConfig interface {
	GetTargetPath(match *Match) (string, error)
}

type downloadConfig struct {
	TargetDir          string
	TargetFileTemplate *template.Template
}

func NewDownloadConfig() DownloadConfig {
	return &downloadConfig{
		TargetDir:          ".",
		TargetFileTemplate: template.Must(template.New("target_file").Parse("{{.ProviderName}} - {{.Info.ID}} - {{.Info.Title}}.{{.Info.Ext}}")),
	}
}

func (c *downloadConfig) GetTargetPath(match *Match) (string, error) {
	args := targetFileTemplateArgs{
		ProviderName: match.ProviderName,
		Info:         match.Source.Info(),
	}
	builder := strings.Builder{}
	if err := c.TargetFileTemplate.Execute(&builder, &args); err != nil {
		return "", err
	} else {
		return builder.String(), nil
	}
}

type targetFileTemplateArgs struct {
	ProviderName string
	Info         SourceInfo
}
