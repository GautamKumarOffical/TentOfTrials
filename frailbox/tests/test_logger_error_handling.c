#define _GNU_SOURCE
#define _DEFAULT_SOURCE

#include "../include/logger.h"

#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

static int read_file(const char *path, char *buffer, size_t buffer_size)
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

static int capture_stderr(const char *path, int *saved_stderr)
{
    int capture_fd = open(path, O_CREAT | O_TRUNC | O_RDWR, 0600);
    if (capture_fd < 0) {
        return -1;
    }

    *saved_stderr = dup(STDERR_FILENO);
    if (*saved_stderr < 0) {
        close(capture_fd);
        return -1;
    }

    if (dup2(capture_fd, STDERR_FILENO) < 0) {
        close(capture_fd);
        close(*saved_stderr);
        return -1;
    }

    close(capture_fd);
    return 0;
}

static void restore_stderr(int saved_stderr)
{
    fflush(stderr);
    dup2(saved_stderr, STDERR_FILENO);
    close(saved_stderr);
}

static int output_contains(const char *path, const char *needle)
{
    char buffer[8192];
    if (read_file(path, buffer, sizeof(buffer)) != 0) {
        return 0;
    }
    return strstr(buffer, needle) != NULL;
}

static int test_open_failure_falls_back_to_stderr(void)
{
    char temp_dir[] = "/tmp/frailbox-logger-open-XXXXXX";
    if (mkdtemp(temp_dir) == NULL) {
        return 1;
    }

    char capture_path[256];
    snprintf(capture_path, sizeof(capture_path), "%s/stderr.log", temp_dir);

    int saved_stderr = -1;
    if (capture_stderr(capture_path, &saved_stderr) != 0) {
        rmdir(temp_dir);
        return 1;
    }

    setenv("LOG_FILE", temp_dir, 1);
    setenv("LOG_LEVEL", "info", 1);
    log_init();
    LOG_INFO("fallback after open failure");
    log_shutdown();

    restore_stderr(saved_stderr);
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");

    int ok = output_contains(capture_path, "failed to open LOG_FILE") &&
             output_contains(capture_path, "falling back to stderr") &&
             output_contains(capture_path, "fallback after open failure");

    unlink(capture_path);
    rmdir(temp_dir);
    return ok ? 0 : 1;
}

static int test_write_failure_falls_back_to_stderr(void)
{
    if (access("/dev/full", W_OK) != 0) {
        return 0;
    }

    char temp_dir[] = "/tmp/frailbox-logger-write-XXXXXX";
    if (mkdtemp(temp_dir) == NULL) {
        return 1;
    }

    char capture_path[256];
    snprintf(capture_path, sizeof(capture_path), "%s/stderr.log", temp_dir);

    int saved_stderr = -1;
    if (capture_stderr(capture_path, &saved_stderr) != 0) {
        rmdir(temp_dir);
        return 1;
    }

    setenv("LOG_FILE", "/dev/full", 1);
    setenv("LOG_LEVEL", "info", 1);
    log_init();
    LOG_INFO("fallback after write failure");
    log_shutdown();

    restore_stderr(saved_stderr);
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");

    int ok = output_contains(capture_path, "failed to flush log file '/dev/full'") &&
             output_contains(capture_path, "falling back to stderr") &&
             output_contains(capture_path, "fallback after write failure");

    unlink(capture_path);
    rmdir(temp_dir);
    return ok ? 0 : 1;
}

int main(void)
{
    if (test_open_failure_falls_back_to_stderr() != 0) {
        fprintf(stderr, "open failure fallback test failed\n");
        return 1;
    }

    if (test_write_failure_falls_back_to_stderr() != 0) {
        fprintf(stderr, "write failure fallback test failed\n");
        return 1;
    }

    return 0;
}
