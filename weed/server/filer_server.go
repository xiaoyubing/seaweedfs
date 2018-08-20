package weed_server

import (
	"net/http"

	"github.com/chrislusf/seaweedfs/weed/filer2"
	_ "github.com/chrislusf/seaweedfs/weed/filer2/cassandra"
	_ "github.com/chrislusf/seaweedfs/weed/filer2/leveldb"
	_ "github.com/chrislusf/seaweedfs/weed/filer2/memdb"
	_ "github.com/chrislusf/seaweedfs/weed/filer2/mysql"
	_ "github.com/chrislusf/seaweedfs/weed/filer2/postgres"
	_ "github.com/chrislusf/seaweedfs/weed/filer2/redis"
	"github.com/chrislusf/seaweedfs/weed/glog"
	"github.com/chrislusf/seaweedfs/weed/msgqueue"
	_ "github.com/chrislusf/seaweedfs/weed/msgqueue/kafka"
	_ "github.com/chrislusf/seaweedfs/weed/msgqueue/log"
	"github.com/chrislusf/seaweedfs/weed/security"
	"github.com/spf13/viper"
)

type FilerOption struct {
	Masters            []string
	Collection         string
	DefaultReplication string
	RedirectOnRead     bool
	DisableDirListing  bool
	MaxMB              int
	SecretKey          string
	DirListingLimit    int
	DataCenter         string
}

type FilerServer struct {
	option *FilerOption
	secret security.Secret
	filer  *filer2.Filer
}

func NewFilerServer(defaultMux, readonlyMux *http.ServeMux, option *FilerOption) (fs *FilerServer, err error) {
	fs = &FilerServer{
		option: option,
	}

	if len(option.Masters) == 0 {
		glog.Fatal("master list is required!")
	}

	fs.filer = filer2.NewFiler(option.Masters)

	go fs.filer.KeepConnectedToMaster()

	loadConfiguration("filer", true)
	v := viper.GetViper()

	fs.filer.LoadConfiguration(v)

	msgqueue.LoadConfiguration(v.Sub("notification"))

	defaultMux.HandleFunc("/favicon.ico", faviconHandler)
	defaultMux.HandleFunc("/", fs.filerHandler)
	if defaultMux != readonlyMux {
		readonlyMux.HandleFunc("/", fs.readonlyFilerHandler)
	}

	return fs, nil
}

func (fs *FilerServer) jwt(fileId string) security.EncodedJwt {
	return security.GenJwt(fs.secret, fileId)
}

func loadConfiguration(configFileName string, required bool) {

	// find a filer store
	viper.SetConfigName(configFileName)     // name of config file (without extension)
	viper.AddConfigPath(".")                // optionally look for config in the working directory
	viper.AddConfigPath("$HOME/.seaweedfs") // call multiple times to add many search paths
	viper.AddConfigPath("/etc/seaweedfs/")  // path to look for the config file in

	glog.V(0).Infof("Reading %s.toml from %s", configFileName, viper.ConfigFileUsed())

	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		glog.V(0).Infof("Reading %s: %v", viper.ConfigFileUsed(), err)
		if required {
			glog.Fatalf("Failed to load %s.toml file from current directory, or $HOME/.seaweedfs/, or /etc/seaweedfs/"+
				"\n\nPlease follow this example and add a filer.toml file to "+
				"current directory, or $HOME/.seaweedfs/, or /etc/seaweedfs/:\n"+
				"    https://github.com/chrislusf/seaweedfs/blob/master/weed/%s.toml\n",
				configFileName, configFileName)
		}
	}

}
