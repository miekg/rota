default: help

help:
	@echo "Available commands:"
	@sed -n '/^[a-zA-Z0-9_]*:/s/:.*//p' <Makefile

buildjs:
	cd cmd/app/static && make build

cleanjs:
	cd cmd/app/static && make clean

clean: cleanjs

devserver: buildjs
	dev_appserver.py cmd/app/app_local.yaml

devserver-remote: buildjs
	dev_appserver.py --host 0.0.0.0 --enable_host_checking no cmd/app/app_local.yaml
