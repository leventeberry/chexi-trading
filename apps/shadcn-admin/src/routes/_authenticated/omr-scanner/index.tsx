import { createFileRoute } from '@tanstack/react-router'
import { OmrScannerPage } from '@/features/omr-scanner/omr-scanner-page'

export const Route = createFileRoute('/_authenticated/omr-scanner/')({
  component: OmrScannerPage,
})
