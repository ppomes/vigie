#!/bin/sh
# Imports a Cachet 2.4 database into this Vigie installation, run with
# docker compose from the repository. The options are those of
# "api import-cachet" (see docs/IMPORT_CACHET.md).
#
# It runs a dry run first, prints its report, and asks before importing.
# With --translate, it starts the Ollama service, pulls the model, and
# stops the service at the end. The model is kept in a volume for the next
# run, and so are the translations already made (in the API container).
#
#   scripts/import-cachet.sh --source-dsn 'cachet:secret@tcp(cachet-db:3306)/cachet' --translate
set -eu

cd "$(dirname "$0")/.."

translate=no
dry_run=no
model=qwen2.5:7b
previous=
for arg in "$@"; do
	case "$arg" in
	--translate | --translate=true) translate=yes ;;
	--dry-run | --dry-run=true) dry_run=yes ;;
	--translator-model=*) model=${arg#*=} ;;
	esac
	if [ "$previous" = "--translator-model" ]; then
		model=$arg
	fi
	previous=$arg
done

if [ "$translate" = yes ]; then
	echo "Starting Ollama..."
	docker compose --profile translate up -d ollama
	trap 'echo "Stopping Ollama..."; docker compose --profile translate stop ollama >/dev/null' EXIT
	tries=0
	until docker compose exec -T ollama ollama list >/dev/null 2>&1; do
		tries=$((tries + 1))
		if [ "$tries" -ge 60 ]; then
			echo "Ollama did not start: see docker compose logs ollama" >&2
			exit 1
		fi
		sleep 1
	done
	echo "Pulling $model (only downloaded the first time)..."
	docker compose exec -T ollama ollama pull "$model"
fi

if [ "$dry_run" = yes ]; then
	docker compose exec -T api ./api import-cachet "$@"
	exit
fi

docker compose exec -T api ./api import-cachet "$@" --dry-run
echo
printf "Nothing was written yet. Import for real? [y/N] "
read -r answer
case "$answer" in
y | Y | yes | o | O | oui) ;;
*)
	echo "Import cancelled."
	exit 1
	;;
esac
docker compose exec -T api ./api import-cachet "$@"
