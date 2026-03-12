.PHONY: install uninstall reinstall test lint format typecheck sync

install:
	uv tool install --editable .

uninstall:
	uv tool uninstall slack-cli

reinstall: uninstall install

sync:
	uv sync

test:
	uv run pytest tests/ -v

lint:
	uv run ruff check src/ tests/

format:
	uv run ruff format src/ tests/

typecheck:
	uv run pyright src/
