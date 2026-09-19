<script setup lang="ts">
import { watch } from "vue"

const props = defineProps(['reviewImg', 'reviewUrl'])
const emit = defineEmits(['closePreImg'])
const close = () => {
    emit('closePreImg')
}
</script>
<template>
    <!-- v-if keeps the viewer (and TDesign's always-rendered trigger element,
         which shows a broken-image error for an empty URL) out of the DOM
         until there is actually an image to preview. -->
    <t-image-viewer v-if="reviewImg && reviewUrl" :visible="reviewImg" closeOnOverlay closeOnEscKeydown @close="close"
        :images="reviewUrl ? [reviewUrl] : []">
    </t-image-viewer>
</template>
<style scoped lang="less"></style>
