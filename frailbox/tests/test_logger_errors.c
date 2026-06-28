#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "../include/logger.h"

static int expect_true(int condition, const char *message)
{
    if (!condition) {
        fprintf(stderr, "FAIL: %s\n", message);
        return 1;
    }
    return 0;
}

int main(void)
{
    int failures = 0;
    char template_path[] = "/tmp/frailbox-logger-XXXXXX";
    int fd = mkstemp(template_path);
    if (fd < 0) {
        perror("mkstemp");
        return 1;
    }
    close(fd);

    setenv("LOG_LEVEL", "info", 1);
    setenv("LOG_FILE", template_path, 1);
    failures += expect_true(log_init() == 0, "log_init succeeds for writable file");
    LOG_INFO("normal file write");
    failures += expect_true(log_get_fallback_count() == 0, "writable file does not trigger fallback");
    log_shutdown();

    setenv("LOG_LEVEL", "info", 1);
    setenv("LOG_FILE", "/tmp/frailbox-missing-dir/logger.log", 1);
    failures += expect_true(log_init() == 0, "log_init succeeds with stderr fallback");
    failures += expect_true(log_get_fallback_count() >= 1, "open failure increments fallback count");
    LOG_WARN("fallback after open failure");
    log_shutdown();

    setenv("LOG_LEVEL", "info", 1);
    setenv("LOG_FILE", "/dev/full", 1);
    unsigned int before = log_get_fallback_count();
    failures += expect_true(log_init() == 0, "log_init succeeds for /dev/full");
    LOG_ERROR("write should fall back from /dev/full");
    failures += expect_true(log_get_fallback_count() > before, "write failure increments fallback count");
    log_shutdown();

    unlink(template_path);
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");

    if (failures == 0) {
        printf("logger error handling tests passed\n");
    }
    return failures == 0 ? 0 : 1;
}
