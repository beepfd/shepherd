package config

type Configuration struct {
	Pprof      PprofConfig   `yaml:"pprof"`
	BTF        BTFConfig     `yaml:"btf"`
	Output     OutputConfig  `yaml:"output"`
	Logging    LoggingConfig `yaml:"logging"`
	ConfigPath string        `yaml:"-"`
}

type PprofConfig struct {
	Enable bool `yaml:"enable"`
}

type BTFConfig struct {
	Kernel   string `yaml:"kernel"`
	ModelDir string `yaml:"model_dir"`
}

type OutputConfig struct {
	Type       OutputType             `yaml:"type"`
	File       FileOutputConfig       `yaml:"file"`
	Stdout     struct{}               `yaml:"stdout"`
	Kafka      KafkaOutputConfig      `yaml:"kafka"`
	Clickhouse ClickhouseOutputConfig `yaml:"clickhouse"`
}

type OutputType string

const (
	OutputTypeFile       OutputType = "file"
	OutputTypeStdout     OutputType = "stdout"
	OutputTypeKafka      OutputType = "kafka"
	OutputTypeClickhouse OutputType = "clickhouse"
)

type ClickhouseOutputConfig struct {
	Port     string `yaml:"port"`
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type FileOutputConfig struct {
	Path string `yaml:"path"`
}

type KafkaOutputConfig struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
}

type LoggingConfig struct {
	ToStderr     bool   `yaml:"to_stderr"`
	AlsoToStderr bool   `yaml:"also_to_stderr"`
	File         string `yaml:"file"`
}
