#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "../include/logger.h"

static int run_invalid_log_file_fallback(void)
{
    setenv("LOG_FILE", "/totally/invalid/path/for/frailbox/logger.log", 1);
    setenv("LOG_LEVEL", "info", 1);

    assert(log_init() == 0);
    assert(log_uses_stderr_fallback() == 1);
    assert(log_last_io_error() != NULL);
    assert(strstr(log_last_io_error(), "falling back to stderr") != NULL);

    LOG_INFO("logger fallback smoke test");
    log_shutdown();
    return 0;
}

static int run_default_stderr_mode(void)
{
    unsetenv("LOG_FILE");
    setenv("LOG_LEVEL", "info", 1);

    assert(log_init() == 0);
    assert(log_uses_stderr_fallback() == 0);
    assert(log_last_io_error() == NULL);

    LOG_INFO("default stderr mode");
    log_shutdown();
    return 0;
}

int main(void)
{
    if (run_default_stderr_mode() != 0) {
        fprintf(stderr, "default stderr mode test failed\n");
        return 1;
    }
    if (run_invalid_log_file_fallback() != 0) {
        fprintf(stderr, "invalid LOG_FILE fallback test failed\n");
        return 1;
    }
    puts("logger error-handling tests passed");
    return 0;
}
