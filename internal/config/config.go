package config

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	// Config 是 config.yaml 的内存映射。mapstructure 标签将 YAML 字段映射到 Go 字段。
	App     AppConfig     `mapstructure:"app"`
	MySQL   MySQLConfig   `mapstructure:"mysql"`
	Cron    CronConfig    `mapstructure:"cron"`
	Engine  EngineConfig  `mapstructure:"engine"`
	Crawler CrawlerConfig `mapstructure:"crawler"`
}

type AppConfig struct {
	// AppConfig 保存服务自身的监听端口、规则脚本和可选代理配置。
	Port      int    `mapstructure:"port"`
	LuaPath   string `mapstructure:"lua_path"`
	Proxy     string `mapstructure:"proxy"`
	PprofPort int    `mapstructure:"pprof_port"`
}
type MySQLConfig struct {
	// MySQLConfig 用于控制连接地址及连接池容量。
	DSN          string `mapstructure:"dsn"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}
type CronConfig struct {
	// Spec 是 robfig/cron 使用的定时表达式。
	Spec string `mapstructure:"spec"`
}

type EngineConfig struct {
	// EngineConfig 决定任务队列、worker 数量以及网络和 Lua 执行的超时阈值。
	WorkerCount       int `mapstructure:"worker_count"`
	TaskQueueSize     int `mapstructure:"task_queue_size"`
	LuaTimeout        int `mapstructure:"lua_timeout"`
	HTTPTimeoutDirect int `mapstructure:"http_timeout_direct"`
	HTTPTimeoutProxy  int `mapstructure:"http_timeout_proxy"`
}

type CrawlerConfig struct {
	// CrawlerConfig 为爬取频率和链接裂变规模预留配置项。
	RequestDelayMin int `mapstructure:"request_delay_min"`
	RequestDelayMax int `mapstructure:"request_delay_max"`
	MaxFissionDepth int `mapstructure:"max_fission_depth"`
	MaxFissionURLs  int `mapstructure:"max_fission_urls"`
}

var (
	// globalConfig 由 Get 提供给全局使用；读写配合 RWMutex，
	// 避免配置热更新时读到正在替换的指针。
	globalConfig *Config
	configMutex  sync.RWMutex
)

func InitConfig() {
	// Viper 从项目根目录下的 configs/config.yaml 读取配置。
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("配置读取失败,请检查configs/config.yaml是否存在:%v", err)
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("配置数据装填失败: %v", err)
	}

	// 首次加载发生在服务启动阶段，此时尚不存在并发读取。
	globalConfig = &cfg

	// 监听配置文件变更。重新反序列化成功后整体替换配置，
	// 这样一次 Get 要么看到旧配置，要么看到完整的新配置。
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("检测到配置文件变化: %s", e.Name)
		var newCfg Config
		if err := viper.Unmarshal(&newCfg); err != nil {
			log.Printf("配置热更新失败: %v\n", err)
			return
		}
		configMutex.Lock()
		globalConfig = &newCfg
		configMutex.Unlock()
		log.Println("配置文件热更新成功")
	})
}

func Get() *Config {
	// 当前调用方只读取配置字段；若未来需要修改返回值，应改为返回副本，
	// 否则会绕开这把锁直接修改全局配置。
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig
}
