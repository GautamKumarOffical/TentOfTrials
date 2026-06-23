CC = gcc
UNAME_S := $(shell uname -s)
CFLAGS = -Wall -Wextra -Wpedantic -std=c2x -O2 -g
CFLAGS += -D_FORTIFY_SOURCE=3
CFLAGS += -fstack-protector-strong
CFLAGS += -fPIE
LDFLAGS =
ifeq ($(UNAME_S),Linux)
LDFLAGS += -pie -z relro -z now
endif

SRCDIR = src
INCDIR = include
BUILDDIR = build

SRCS = $(wildcard $(SRCDIR)/*.c) main.c
OBJS = $(patsubst %.c, $(BUILDDIR)/%.o, $(SRCS))
DEPS = $(OBJS:.o=.d)

TARGET = frailbox
BUDDY_TEST = $(BUILDDIR)/test_buddy

.PHONY: all clean test-buddy

all: $(TARGET)

$(TARGET): $(OBJS)
	$(CC) $(CFLAGS) -I$(INCDIR) $^ -o $@ $(LDFLAGS)

$(BUILDDIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -I$(INCDIR) -MMD -MP -c $< -o $@

$(BUDDY_TEST): tests/test_buddy.c $(SRCDIR)/buddy.c $(INCDIR)/buddy.h
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -I$(INCDIR) tests/test_buddy.c $(SRCDIR)/buddy.c -o $@ $(LDFLAGS)

-include $(DEPS)

clean:
	rm -rf $(BUILDDIR) $(TARGET)

distclean: clean
	rm -rf *.o *.d

test: $(TARGET)
	./$(TARGET) --sandbox-type seccomp --memory-limit 64 --verbose

test-buddy: $(BUDDY_TEST)
	./$(BUDDY_TEST)

valgrind: $(TARGET)
	valgrind --leak-check=full --show-leak-kinds=all ./$(TARGET)

.PHONY: all clean distclean test test-buddy valgrind
