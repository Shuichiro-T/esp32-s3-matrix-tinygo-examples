TARGET  ?= esp32s3-generic
EXAMPLE ?= 01_color_check
PORT    ?=

EXAMPLES := $(notdir $(wildcard examples/*))
PORT_FLAG := $(if $(PORT),-port $(PORT),)

.PHONY: build flash monitor build-all clean

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

clean:
	rm -rf build
