#ifndef FRAILBOX_BUDDY_H
#define FRAILBOX_BUDDY_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct buddy_stats {
    uint64_t total_allocated;
    uint64_t total_freed;
    uint64_t peak_usage;
    uint64_t current_usage;
    uint64_t allocation_count;
    uint64_t deallocation_count;
    uint64_t block_count;
    uint64_t free_block_count;
    double fragmentation_ratio;
} buddy_stats_t;

typedef struct buddy_allocator buddy_allocator_t;

buddy_allocator_t *buddy_create(size_t total_size);
void buddy_destroy(buddy_allocator_t *buddy);

void *buddy_alloc(buddy_allocator_t *buddy, size_t size);
void  buddy_free(buddy_allocator_t *buddy, void *ptr);

buddy_stats_t buddy_stats(const buddy_allocator_t *buddy);
size_t buddy_total_capacity(const buddy_allocator_t *buddy);
int buddy_contains(const buddy_allocator_t *buddy, const void *ptr);

#ifdef __cplusplus
}
#endif

#endif
