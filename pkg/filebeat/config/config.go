package config

import (
	"fmt"
)

var (
	DefaultConfig = Config{
		Cluster:            "",
		Address:            "",
		Database:           "",
		Username:           "",
		Password:           "",
		WriteTimeout:       "10",
		ReadTimeout:        "10",
		AsyncInsert:        false,
		WaitForAsyncInsert: false,
		ForceNumberFields:  []string{},
	}
)

type Config struct {
	Cluster            string   `config:"cluster"`
	Address            string   `config:"address"`
	Database           string   `config:"database"`
	Username           string   `config:"username"`
	Password           string   `config:"password"`
	WriteTimeout       string   `config:"write_timeout"`
	ReadTimeout        string   `config:"read_timeout"`
	AsyncInsert        bool     `config:"async_insert"`
	WaitForAsyncInsert bool     `config:"wait_for_async_insert"`
	ForceNumberFields  []string `config:"force_number_fields"`
}

func (c *Config) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("Address is required")
	}

	if c.Database == "" {
		return fmt.Errorf("Database is required")
	}

	return nil
}
