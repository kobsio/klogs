package filebeat

import (
	"github.com/kobsio/klogs/pkg/clickhouse"
	"github.com/kobsio/klogs/pkg/filebeat/config"
	"github.com/kobsio/klogs/pkg/filebeat/output"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"go.uber.org/zap"
)

func init() {
	outputs.RegisterType("clickhouse", makeClickhouse)
}

func makeClickhouse(_ outputs.IndexManager, beat beat.Info, observer outputs.Observer, cfg *common.Config) (outputs.Group, error) {
	config := config.DefaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return outputs.Fail(err)
	}

	log := logp.NewLogger("clickhouse")
	log.Infow("Clickhouse configuration", zap.String("cluster", config.Cluster), zap.String("address", config.Address), zap.String("username", config.Username), zap.String("password", "*****"), zap.String("database", config.Database), zap.String("writeTimeout", config.WriteTimeout), zap.String("readTimeout", config.ReadTimeout))

	clickhouseClient, err := clickhouse.NewClient(config.Address, config.Username, config.Password, config.Database, config.WriteTimeout, config.ReadTimeout, config.AsyncInsert, config.WaitForAsyncInsert)
	if err != nil {
		return outputs.Fail(err)
	}

	outputClient := output.NewClient(log, observer, config.Cluster, config.Address, config.ForceNumberFields, clickhouseClient)

	return outputs.Success(-1, 3, outputClient)
}
