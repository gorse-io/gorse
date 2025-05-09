SOURCES = munit.c floats_test.c

ifeq ($(shell uname -m),x86_64)
	SOURCES += floats_avx.c floats_avx512.c
	CFLAGS = -O3 -mavx -mavx512f -mavx512dq
else ifeq ($(shell uname -m),aarch64)
	SOURCES += floats_neon.c floats_sve2.c
	CFLAGS = -O3 -march=armv8-a+sve
endif

OBJECTS		= $(SOURCES:.c=.o)
DEPENDENCES	= $(SOURCES:.c=.d)
EXECUTE		= floats_test

$(EXECUTE): $(OBJECTS) 
	$(CC) $(OBJECTS) -lm -o $(EXECUTE)

test: $(EXECUTE)
	./${EXECUTE}

clean:
	rm $(OBJECTS) $(DEPENDENCES) $(EXECUTE)

-include $(DEPENDENCES)

%.d: %.c
	@set -e; \
	rm -f $@; \
	$(CC) $(CFLAGS) -MM -MT $(@:.d=.o) $< > $@.$$$$; \
	sed 's,\($*\)\.o[ :]*,\1.o $@: ,g' $@.$$$$ > $@; \
	rm -f $@.$$$$
