package nvidia

import (
	"io/ioutil"
	"runtime"

	log "github.com/astaxie/beego/logs"
)

func StackTrace(all bool) string {
	buf := make([]byte, 10240)

	for {
		size := runtime.Stack(buf, all)

		if size == len(buf) {
			buf = make([]byte, len(buf)<<1)
			continue
		}
		break

	}

	return string(buf)
}

func coredump(fileName string) {
	log.Info("Dump stacktrace to ", fileName)
	ioutil.WriteFile(fileName, []byte(StackTrace(true)), 0644)
}
