package configure

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func checkErr(err error) {
	if err != nil {
		logrus.WithError(err).Fatal("config")
	}
}

func New() *Config {
	config := viper.New()

	// Default config
	b, _ := json.Marshal(Config{
		ConfigFile: "config.yaml",
	})
	tmp := viper.New()
	defaultConfig := bytes.NewReader(b)
	tmp.SetConfigType("json")
	checkErr(tmp.ReadConfig(defaultConfig))
	checkErr(config.MergeConfigMap(viper.AllSettings()))

	pflag.String("config", "config.yaml", "Config file location")
	pflag.Bool("noheader", false, "Disable the startup header")
	pflag.Parse()
	checkErr(config.BindPFlags(pflag.CommandLine))

	// File
	config.SetConfigFile(config.GetString("config"))
	config.AddConfigPath(".")
	checkErr(config.ReadInConfig())
	checkErr(config.MergeInConfig())

	BindEnvs(config, Config{})

	// Environment
	config.AutomaticEnv()
	config.SetEnvPrefix("TAXES")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.AllowEmptyEnv(true)

	// Print final config
	c := &Config{}
	checkErr(config.Unmarshal(&c))

	initLogging(c.Level)

	return c
}

func BindEnvs(config *viper.Viper, iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			continue
		}
		switch v.Kind() {
		case reflect.Struct:
			BindEnvs(config, v.Interface(), append(parts, tv)...)
		default:
			_ = config.BindEnv(strings.Join(append(parts, tv), "."))
		}
	}
}

type Config struct {
	Level      string `mapstructure:"level" json:"level"`
	ConfigFile string `mapstructure:"config" json:"config"`
	NoHeader   bool   `mapstructure:"noheader" json:"noheader"`

	Redis struct {
		Username   string   `mapstructure:"username" json:"username"`
		Password   string   `mapstructure:"password" json:"password"`
		MasterName string   `mapstructure:"master_name" json:"master_name"`
		Addresses  []string `mapstructure:"addresses" json:"addresses"`
		Database   int      `mapstructure:"database" json:"database"`
		Sentinel   bool     `mapstructure:"sentinel" json:"sentinel"`
	} `mapstructure:"redis" json:"redis"`

	Mongo struct {
		URI      string `mapstructure:"uri" json:"uri"`
		Database string `mapstructure:"database" json:"database"`
		Direct   bool   `mapstructure:"direct" json:"direct"`
	} `mapstructure:"mongo" json:"mongo"`

	Twitch struct {
		ClientID      string `mapstructure:"client_id" json:"client_id"`
		ClientSecret  string `mapstructure:"client_secret" json:"client_secret"`
		RedirectURI   string `mapstructure:"redirect_uri" json:"redirect_uri"`
		WebhookSecret string `mapstructure:"webhook_secret" json:"webhook_secret"`
	} `mapstructure:"twitch" json:"twitch"`

	Frontend struct {
		CookieSecure bool   `mapstructure:"cookie_secure" json:"cookie_secure"`
		CookieDomain string `mapstructure:"cookie_domain" json:"cookie_domain"`
		WebsiteURL   string `mapstructure:"website_url" json:"website_url"`
	} `mapstructure:"frontend" json:"frontend"`

	API struct {
		Bind string `mapstructure:"bind" json:"bind"`
	} `mapstructure:"api" json:"api"`

	Health struct {
		Enabled bool   `mapstructure:"enabled" json:"enabled"`
		Bind    string `mapstructure:"bind" json:"bind"`
	} `mapstructure:"health" json:"health"`
}
