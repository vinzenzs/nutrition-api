"""Entrypoint: load config (fail-fast), install redacting JSON logging, serve.

Run with ``python -m garmin_bridge`` (the Dockerfile CMD). Honours PORT and
LOG_LEVEL; binds 0.0.0.0 so the k8s Service can reach it.
"""

from __future__ import annotations

import os

import uvicorn

from . import config as config_module
from . import logging_setup
from .app import create_app


def main() -> None:
    cfg = config_module.load()
    logging_setup.configure(
        level=os.environ.get("LOG_LEVEL", "INFO"),
        secrets=cfg.sensitive_values,
    )
    app = create_app(cfg)
    uvicorn.run(
        app,
        host="0.0.0.0",  # noqa: S104 — bound inside the cluster network only
        port=int(os.environ.get("PORT", "8080")),
        log_config=None,  # keep our JSON logging, not uvicorn's default
    )


if __name__ == "__main__":
    main()
