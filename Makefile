TARGET  ?= esp32s3-generic
EXAMPLE ?= 01_color_check
PORT    ?=

EXAMPLES := $(notdir $(wildcard examples/*))
PORT_FLAG := $(if $(PORT),-port $(PORT),)

.PHONY: build flash monitor build-all serve model clean

## make build EXAMPLE=02_rainbow
build:
	@mkdir -p build
	tinygo build -target $(TARGET) -o build/$(EXAMPLE).bin ./examples/$(EXAMPLE)

## make flash EXAMPLE=02_rainbow [PORT=/dev/cu.usbmodem1101]
flash:
	tinygo flash -target $(TARGET) $(PORT_FLAG) -monitor ./examples/$(EXAMPLE)

monitor:
	tinygo monitor $(PORT_FLAG)

build-all:
	@mkdir -p build
	@for e in $(EXAMPLES); do \
		echo "== $$e"; \
		tinygo build -target $(TARGET) -o build/$$e.bin ./examples/$$e || exit 1; \
	done

## ブラウザ側ページを http://localhost:8080 で配信する
## make serve EXAMPLE=07_music_ripple（EXAMPLE に web/ がなければ 06_ble_color）
## 07_music_ripple では、ビート検出のモデルがなければ先にダウンロードして変換する（初回のみ）
WEB_DIR := $(if $(wildcard examples/$(EXAMPLE)/web),examples/$(EXAMPLE)/web,examples/06_ble_color/web)
FETCH_MODEL := $(wildcard examples/$(EXAMPLE)/tools/fetch_model.sh)
serve:
	$(if $(FETCH_MODEL),sh $(FETCH_MODEL))
	python3 -m http.server 8080 --bind 127.0.0.1 -d $(WEB_DIR)

## Beat This! のモデルを用意する: make model [MODEL=final0]
MODEL ?= small0
model:
	sh examples/07_music_ripple/tools/fetch_model.sh $(MODEL)

clean:
	rm -rf build
