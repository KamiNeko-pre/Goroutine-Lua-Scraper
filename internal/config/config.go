package config

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	App   AppConfig   `mapstructure:"app"`
	MySQL MySQLConfig `mapstructure:"mysql"`
	Cron CronConfig   `mapstructure:"cron"`
}

type AppConfig struct {
	Port    int    `mapstructure:"port"`
	LuaPath string `mapstructure:"lua_path"`
	Proxy   string  `mapstructure:"proxy"`
}
type MySQLConfig struct {
	DSN          string `mapstructure:"dsn"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}
type CronConfig struct{
	Spec string         `mapstructure:"spec"`
}

var(
	globalConfig *Config
	configMutex  sync.RWMutex
)


func InitConfig()  {
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

	globalConfig=&cfg

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
	log.Printf("检测到配置文件变化: %s",e.Name)
	var newCfg Config
	if err:=viper.Unmarshal(&newCfg);err!=nil{
		log.Printf("配置热更新失败: %v\n",err)
		return
	}
	configMutex.Lock()
	globalConfig=&newCfg
	configMutex.Unlock()
	log.Println("配置文件热更新成功")
	})
}

func Get() *Config{
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig
}