#define _GNU_SOURCE

#include "../include/logger.h"

#include <assert.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

struct stderr_capture {
    int saved_fd;
    char path[256];
};

static void capture_stderr_begin(struct stderr_capture *capture)
{
    snprintf(capture->path, sizeof(capture->path),
             "/tmp/frailbox-logger-errors-%ld-XXXXXX", (long)getpid());

    int fd = mkstemp(capture->path);
    assert(fd >= 0);

    fflush(stderr);
    capture->saved_fd = dup(STDERR_FILENO);
    assert(capture->saved_fd >= 0);
    assert(dup2(fd, STDERR_FILENO) >= 0);
    close(fd);
}

static char *capture_stderr_end(struct stderr_capture *capture)
{
    fflush(stderr);
    assert(dup2(capture->saved_fd, STDERR_FILENO) >= 0);
    close(capture->saved_fd);

    FILE *fp = fopen(capture->path, "rb");
    assert(fp != NULL);
    assert(fseek(fp, 0, SEEK_END) == 0);
    long size = ftell(fp);
    assert(size >= 0);
    rewind(fp);

    char *data = calloc((size_t)size + 1, 1);
    assert(data != NULL);
    assert(fread(data, 1, (size_t)size, fp) == (size_t)size);
    fclose(fp);
    unlink(capture->path);
    return data;
}

static void configure_logger(const char *log_file)
{
    setenv("LOG_LEVEL", "debug", 1);
    setenv("LOG_NO_TIMESTAMPS", "1", 1);
    setenv("LOG_MODULE", "logger-test", 1);
    setenv("LOG_FILE", log_file, 1);
}

static void test_open_failure_falls_back_to_stderr(void)
{
    struct stderr_capture capture;
    capture_stderr_begin(&capture);

    configure_logger("/tmp/frailbox-missing-directory/logger.log");
    assert(log_init() == 0);
    LOG_ERROR("open fallback message");
    log_shutdown();

    char *stderr_text = capture_stderr_end(&capture);
    assert(strstr(stderr_text, "open failed for log file") != NULL);
    assert(strstr(stderr_text, "falling back to stderr") != NULL);
    assert(strstr(stderr_text, "open fallback message") != NULL);
    free(stderr_text);
}

static void test_write_failure_falls_back_to_stderr(void)
{
    if (access("/dev/full", W_OK) != 0) {
        fprintf(stderr, "skipping /dev/full write fallback test on this platform\n");
        return;
    }

    struct stderr_capture capture;
    capture_stderr_begin(&capture);

    configure_logger("/dev/full");
    assert(log_init() == 0);
    LOG_ERROR("write fallback message");
    log_shutdown();

    char *stderr_text = capture_stderr_end(&capture);
    assert(strstr(stderr_text, "write failed for log file") != NULL);
    assert(strstr(stderr_text, "falling back to stderr") != NULL);
    assert(strstr(stderr_text, "write fallback message") != NULL);
    free(stderr_text);
}

int main(void)
{
    test_open_failure_falls_back_to_stderr();
    test_write_failure_falls_back_to_stderr();
    return 0;
}
