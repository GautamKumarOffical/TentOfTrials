#include "buddy.h"

#include <errno.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct buddy_block {
    uint8_t order;
    uint8_t free;
    size_t requested_size;
    struct buddy_block *next;
    struct buddy_block *prev;
} buddy_block_t;

struct buddy_allocator {
    unsigned char *base;
    size_t capacity;
    uint8_t min_order;
    uint8_t max_order;
    buddy_block_t **free_lists;
    buddy_stats_t stats;
};

static int is_power_of_two(size_t value) {
    return value != 0 && (value & (value - 1u)) == 0;
}

static size_t next_power_of_two(size_t value) {
    if (value <= 1u) {
        return 1u;
    }

    value--;
    for (size_t shift = 1u; shift < sizeof(size_t) * 8u; shift <<= 1u) {
        value |= value >> shift;
    }
    return value + 1u;
}

static uint8_t order_for_size(size_t size) {
    uint8_t order = 0;
    size_t block_size = 1u;

    while (block_size < size) {
        block_size <<= 1u;
        order++;
    }

    return order;
}

static size_t size_for_order(uint8_t order) {
    return (size_t)1u << order;
}

static void list_push(buddy_allocator_t *allocator, buddy_block_t *block) {
    buddy_block_t **head = &allocator->free_lists[block->order];

    block->free = 1u;
    block->requested_size = 0u;
    block->prev = NULL;
    block->next = *head;
    if (*head) {
        (*head)->prev = block;
    }
    *head = block;
}

static void list_remove(buddy_allocator_t *allocator, buddy_block_t *block) {
    buddy_block_t **head = &allocator->free_lists[block->order];

    if (block->prev) {
        block->prev->next = block->next;
    } else if (*head == block) {
        *head = block->next;
    }

    if (block->next) {
        block->next->prev = block->prev;
    }

    block->next = NULL;
    block->prev = NULL;
    block->free = 0u;
}

static buddy_block_t *block_at(const buddy_allocator_t *allocator, size_t offset) {
    return (buddy_block_t *)(void *)(allocator->base + offset);
}

static size_t block_offset(const buddy_allocator_t *allocator, const buddy_block_t *block) {
    return (size_t)((const unsigned char *)(const void *)block - allocator->base);
}

static void refresh_free_stats(buddy_allocator_t *allocator) {
    size_t free_bytes = 0u;
    size_t largest = 0u;

    for (uint8_t order = allocator->min_order; order <= allocator->max_order; order++) {
        size_t block_size = size_for_order(order);
        for (buddy_block_t *block = allocator->free_lists[order]; block; block = block->next) {
            free_bytes += block_size;
            if (block_size > largest) {
                largest = block_size;
            }
        }
    }

    allocator->stats.free_bytes = free_bytes;
    allocator->stats.largest_free_block = largest;
    allocator->stats.fragmentation_ratio =
        free_bytes == 0u ? 0.0 : 1.0 - ((double)largest / (double)free_bytes);
}

buddy_allocator_t *buddy_create(size_t capacity_bytes) {
    size_t min_capacity = BUDDY_MIN_BLOCK_SIZE;

    if (capacity_bytes < min_capacity) {
        capacity_bytes = min_capacity;
    }

    capacity_bytes = next_power_of_two(capacity_bytes);
    if (!is_power_of_two(capacity_bytes)) {
        errno = EOVERFLOW;
        return NULL;
    }

    buddy_allocator_t *allocator = calloc(1u, sizeof(*allocator));
    if (!allocator) {
        return NULL;
    }

    allocator->capacity = capacity_bytes;
    allocator->min_order = order_for_size(BUDDY_MIN_BLOCK_SIZE);
    allocator->max_order = order_for_size(capacity_bytes);

    allocator->free_lists = calloc((size_t)allocator->max_order + 1u, sizeof(*allocator->free_lists));
    if (!allocator->free_lists) {
        free(allocator);
        return NULL;
    }

    int rc = posix_memalign((void **)&allocator->base, BUDDY_MIN_BLOCK_SIZE, allocator->capacity);
    if (rc != 0) {
        free(allocator->free_lists);
        free(allocator);
        errno = rc;
        return NULL;
    }
    memset(allocator->base, 0, allocator->capacity);

    buddy_block_t *root = block_at(allocator, 0u);
    root->order = allocator->max_order;
    list_push(allocator, root);

    allocator->stats.capacity_bytes = allocator->capacity;
    refresh_free_stats(allocator);

    return allocator;
}

void buddy_destroy(buddy_allocator_t *allocator) {
    if (!allocator) {
        return;
    }

    free(allocator->base);
    free(allocator->free_lists);
    memset(&allocator->stats, 0, sizeof(allocator->stats));
    free(allocator);
}

void *buddy_alloc(buddy_allocator_t *allocator, size_t size) {
    if (!allocator || size == 0u) {
        return NULL;
    }

    if (size > SIZE_MAX - sizeof(buddy_block_t)) {
        return NULL;
    }

    size_t required = size + sizeof(buddy_block_t);
    if (required < BUDDY_MIN_BLOCK_SIZE) {
        required = BUDDY_MIN_BLOCK_SIZE;
    }

    size_t rounded_required = next_power_of_two(required);
    if (rounded_required == 0u || rounded_required > allocator->capacity) {
        return NULL;
    }

    uint8_t needed_order = order_for_size(rounded_required);
    if (needed_order < allocator->min_order) {
        needed_order = allocator->min_order;
    }

    uint8_t order = needed_order;
    while (order <= allocator->max_order && allocator->free_lists[order] == NULL) {
        order++;
    }

    if (order > allocator->max_order) {
        return NULL;
    }

    buddy_block_t *block = allocator->free_lists[order];
    list_remove(allocator, block);

    while (order > needed_order) {
        order--;
        size_t half_size = size_for_order(order);
        buddy_block_t *right = block_at(allocator, block_offset(allocator, block) + half_size);
        right->order = order;
        list_push(allocator, right);
        block->order = order;
        allocator->stats.split_count++;
    }

    block->free = 0u;
    block->requested_size = size;
    block->next = NULL;
    block->prev = NULL;

    allocator->stats.allocated_bytes += size;
    allocator->stats.reserved_bytes += size_for_order(block->order);
    allocator->stats.allocation_count++;
    refresh_free_stats(allocator);

    return (unsigned char *)(void *)block + sizeof(*block);
}

void buddy_free(buddy_allocator_t *allocator, void *ptr) {
    if (!allocator || !ptr) {
        return;
    }

    buddy_block_t *block = (buddy_block_t *)(void *)((unsigned char *)ptr - sizeof(*block));
    if (!buddy_contains(allocator, block) || block->free) {
        return;
    }

    allocator->stats.allocated_bytes -= block->requested_size;
    allocator->stats.reserved_bytes -= size_for_order(block->order);
    allocator->stats.deallocation_count++;

    block->requested_size = 0u;
    block->free = 1u;

    while (block->order < allocator->max_order) {
        size_t block_size = size_for_order(block->order);
        size_t offset = block_offset(allocator, block);
        size_t buddy_offset = offset ^ block_size;

        if (buddy_offset >= allocator->capacity) {
            break;
        }

        buddy_block_t *buddy = block_at(allocator, buddy_offset);
        if (!buddy->free || buddy->order != block->order) {
            break;
        }

        list_remove(allocator, buddy);
        if (buddy_offset < offset) {
            block = buddy;
        }

        block->order++;
        block->free = 1u;
        block->requested_size = 0u;
        block->next = NULL;
        block->prev = NULL;
        allocator->stats.coalesce_count++;
    }

    list_push(allocator, block);
    refresh_free_stats(allocator);
}

buddy_stats_t buddy_stats(const buddy_allocator_t *allocator) {
    if (!allocator) {
        buddy_stats_t empty = {0};
        return empty;
    }

    return allocator->stats;
}

int buddy_contains(const buddy_allocator_t *allocator, const void *ptr) {
    if (!allocator || !ptr) {
        return 0;
    }

    const unsigned char *byte_ptr = ptr;
    return byte_ptr >= allocator->base && byte_ptr < allocator->base + allocator->capacity;
}
