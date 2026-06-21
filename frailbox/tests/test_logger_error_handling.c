#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "../include/logger.h"

#define CAPTURE_SIZE 8192

static int failures = 0;

static void fail(const char *message)
{
    fprintf(stderr, "FAIL: %s\n", message);
    failures++;
}

static int read_capture(const char *path, char *buffer, size_t buffer_size)
{
    FILE *file = fopen(path, "r");
    if (file == NULL) {
        return -1;
    }

    size_t bytes = fread(buffer, 1, buffer_size - 1, file);
    buffer[bytes] = '\0';
    fclose(file);
    return 0;
}

static void expect_contains(const char *text, const char *needle, const char *context)
{
    if (strstr(text, needle) == NULL) {
        fprintf(stderr, "Missing '%s' in %s\n", needle, context);
        failures++;
    }
}

static int redirect_stderr_to(const char *path, int *saved_stderr)
{
    fflush(stderr);

    *saved_stderr = dup(STDERR_FILENO);
    if (*saved_stderr < 0) {
        return -1;
    }

    int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0600);
    if (fd < 0) {
        close(*saved_stderr);
        *saved_stderr = -1;
        return -1;
    }

    if (dup2(fd, STDERR_FILENO) < 0) {
        close(fd);
        close(*saved_stderr);
        *saved_stderr = -1;
        return -1;
    }

    close(fd);
    clearerr(stderr);
    return 0;
}

static void restore_stderr(int saved_stderr)
{
    fflush(stderr);
    dup2(saved_stderr, STDERR_FILENO);
    close(saved_stderr);
    clearerr(stderr);
}

static void clear_logger_env(void)
{
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");
    unsetenv("LOG_MODULE");
    unsetenv("LOG_SOURCE_INFO");
    unsetenv("LOG_NO_TIMESTAMPS");
}

static int test_open_failure_uses_stderr(void)
{
    char template[] = "/tmp/frailbox-logger-open-XXXXXX";
    char *dir = mkdtemp(template);
    if (dir == NULL) {
        fail("mkdtemp failed");
        return -1;
    }

    char capture_path[512];
    char log_path[512];
    snprintf(capture_path, sizeof(capture_path), "%s/stderr.txt", dir);
    snprintf(log_path, sizeof(log_path), "%s/missing/log.txt", dir);

    clear_logger_env();
    setenv("LOG_FILE", log_path, 1);
    setenv("LOG_LEVEL", "info", 1);
    setenv("LOG_NO_TIMESTAMPS", "1", 1);

    int saved_stderr = -1;
    if (redirect_stderr_to(capture_path, &saved_stderr) != 0) {
        fail("redirect_stderr_to failed");
        return -1;
    }

    log_init();
    LOG_INFO("fallback-open-check");
    log_shutdown();
    restore_stderr(saved_stderr);

    char capture[CAPTURE_SIZE];
    if (read_capture(capture_path, capture, sizeof(capture)) != 0) {
        fail("failed to read open-failure capture");
    } else {
        expect_contains(capture, "failed to open log file", "open failure capture");
        expect_contains(capture, "using stderr fallback", "open failure capture");
        expect_contains(capture, "fallback-open-check", "open failure capture");
    }

    unlink(capture_path);
    rmdir(dir);
    clear_logger_env();
    return 0;
}

static int test_write_failure_uses_stderr(void)
{
    if (access("/dev/full", W_OK) != 0) {
        printf("SKIP: /dev/full is unavailable; write failure fallback not exercised\n");
        return 0;
    }

    char template[] = "/tmp/frailbox-logger-write-XXXXXX";
    char *dir = mkdtemp(template);
    if (dir == NULL) {
        fail("mkdtemp failed");
        return -1;
    }

    char capture_path[512];
    snprintf(capture_path, sizeof(capture_path), "%s/stderr.txt", dir);

    clear_logger_env();
    setenv("LOG_FILE", "/dev/full", 1);
    setenv("LOG_LEVEL", "info", 1);
    setenv("LOG_NO_TIMESTAMPS", "1", 1);

    int saved_stderr = -1;
    if (redirect_stderr_to(capture_path, &saved_stderr) != 0) {
        fail("redirect_stderr_to failed");
        return -1;
    }

    log_init();
    LOG_INFO("fallback-write-check");
    log_shutdown();
    restore_stderr(saved_stderr);

    char capture[CAPTURE_SIZE];
    if (read_capture(capture_path, capture, sizeof(capture)) != 0) {
        fail("failed to read write-failure capture");
    } else {
        expect_contains(capture, "failed to write to log file", "write failure capture");
        expect_contains(capture, "using stderr fallback", "write failure capture");
        expect_contains(capture, "fallback-write-check", "write failure capture");
    }

    unlink(capture_path);
    rmdir(dir);
    clear_logger_env();
    return 0;
}

int main(void)
{
    test_open_failure_uses_stderr();
    test_write_failure_uses_stderr();

    if (failures != 0) {
        fprintf(stderr, "%d logger error-handling checks failed\n", failures);
        return 1;
    }

    printf("logger error-handling checks passed\n");
    return 0;
}
