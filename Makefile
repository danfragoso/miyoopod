.PHONY: go
go:
	@go run scripts/build-inject.go
	CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 GOARCH=arm GOOS=linux go build -o App/MiyooPod/MiyooPod src/*.go

.PHONY: install-hooks
install-hooks:
	@echo "Installing git hooks..."
	@chmod +x .git-hooks/pre-commit
	@mkdir -p $$(git rev-parse --git-dir)/hooks
	@cp .git-hooks/pre-commit $$(git rev-parse --git-dir)/hooks/pre-commit
	@chmod +x $$(git rev-parse --git-dir)/hooks/pre-commit
	@echo "✓ Pre-commit hook installed"

.PHONY: package
package:
	@echo "Creating MiyooPod release package..."
	@read -p "Enter version (e.g., 1.0.0):" VERSION; \
	if [ -z "$$VERSION" ]; then \
		echo "Error: Version cannot be empty"; \
		exit 1; \
	fi; \
	echo "Updating version to $$VERSION..."; \
	go run scripts/update-version.go $$VERSION; \
	go run scripts/build-inject.go; \
	echo "Creating release directory..."; \
	mkdir -p releases; \
	echo "Packaging release..."; \
	cd App && zip -r ../releases/MiyooPod-$$VERSION.zip MiyooPod && cd ..; \
	cp releases/MiyooPod-$$VERSION.zip releases/MiyooPod.zip; \
	echo "✓ Release package created: releases/MiyooPod-$$VERSION.zip"; \
	echo "✓ Latest version copied to: releases/MiyooPod.zip"
