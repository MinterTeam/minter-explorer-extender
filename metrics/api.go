package metrics

import (
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
)

type Api struct {
	Host string
	Port int
}

func New(host string, port int) *Api {
	return &Api{host, port}
}

func (api Api) GetLink() string {
	return api.Host + ":" + strconv.Itoa(api.Port)
}

func (api Api) Run() {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(api.GetLink(), nil)
	helpers.HandleError(err)
}
