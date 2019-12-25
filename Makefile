version := 0.0.0

.PHONY: help
help:    ## show this help
	@echo "Commands:"
	@echo "----------------------------------------------------------------"
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
	@echo "----------------------------------------------------------------"

bootstrap: ## install bumpversion
	@brew install bumpversion

bumpversion-patch: ## bump version by 1 patch
	bumpversion patch --allow-dirty

tag: ## tag the repo (remember to commit)
	git tag v$(version)

push: ## push to remote master
	git push origin master
	git push origin v$(version)