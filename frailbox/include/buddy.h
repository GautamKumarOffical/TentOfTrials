#ifndef FRAILBOX_BUDDY_H
#define FRAILBOX_BUDDY_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define BUDDY_MIN_BLOCK_SIZE 64u

typedef struct buddy_allocator buddy_allocator_t;

typedef struct buddy_stats {
    size_t   capacity_bytes;
    size_t   allocated_bytes;
    size_t   reserved_bytes;
    size_t   free_bytes;
    size_t   largest_free_block;
    uint64_t allocation_count;
    uint64_t deallocation_count;
    uint64_t split_count;
    uint64_t coalesce_count;
    double   fragmentation_ratio;
} buddy_stats_t;

buddy_allocator_t *buddy_create(size_t capacity_bytes);
void               buddy_destroy(buddy_allocator_t *allocator);

void *buddy_alloc(buddy_allocator_t *allocator, size_t size);
void  buddy_free(buddy_allocator_t *allocator, void *ptr);

buddy_stats_t buddy_stats(const buddy_allocator_t *allocator);
int           buddy_contains(const buddy_allocator_t *allocator, const void *ptr);

#ifdef __cplusplus
}
#endif

#endif
