# Slim, non-root image for the garmin-bridge service.
# Two stages: install deps into a venv, then copy into a clean runtime layer so
# build tooling never ships in the final image.
FROM python:3.11-slim AS build

ENV PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app
COPY requirements.txt ./
RUN python -m venv /venv \
    && /venv/bin/pip install --no-cache-dir -r requirements.txt

FROM python:3.11-slim AS runtime

ENV PATH="/venv/bin:$PATH" \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PORT=8080

# Non-root runtime user.
RUN useradd --create-home --uid 65532 --shell /usr/sbin/nologin bridge

WORKDIR /app
COPY --from=build /venv /venv
COPY garmin_bridge ./garmin_bridge

USER 65532
EXPOSE 8080

# Module entrypoint loads config (fail-fast), installs redacting JSON logging,
# and serves uvicorn on $PORT.
CMD ["python", "-m", "garmin_bridge"]
