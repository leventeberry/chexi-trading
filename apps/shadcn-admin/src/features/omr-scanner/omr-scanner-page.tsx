import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AlertCircle, Loader2 } from 'lucide-react'
import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { getGoApiBaseUrl } from '@/lib/goapi-base-url'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

export type MarketTicker = {
  source: string
  type: string
  product_id: string
  price: number
  open_24h: number
  volume_24h: number
  low_24h: number
  high_24h: number
  best_bid: number
  best_ask: number
  side: string
  time: string
  received_at: string
}

export type OmrScoreResult = {
  final_score: number
  label: string
  category_scores: {
    liquidity: number
    trend_quality: number
    pullback_quality: number
    exhaustion: number
    risk_setup: number
    final_reversal: number
  }
  reasons: string[]
  warnings: string[]
  failed_filters: string[]
}

const VOLUME_TRENDS = ['rising', 'flat', 'falling'] as const
const CANDLE_SIGNALS = [
  'bullish_reversal',
  'neutral',
  'bearish_continuation',
] as const

function apiUrl(path: string): string {
  const base = getGoApiBaseUrl()
  return `${base}${path.startsWith('/') ? path : `/${path}`}`
}

async function readApiError(res: Response): Promise<string> {
  try {
    const j: unknown = await res.json()
    if (
      j &&
      typeof j === 'object' &&
      'error' in j &&
      typeof (j as { error: unknown }).error === 'string'
    ) {
      return (j as { error: string }).error
    }
  } catch {
    /* ignore */
  }
  return res.statusText || `HTTP ${res.status}`
}

function buildThesisFromScore(score: OmrScoreResult): string {
  const parts: string[] = [`OMR label: ${score.label}.`]
  const reasons = score.reasons.slice(0, 8)
  if (reasons.length > 0) {
    parts.push(`Reasons: ${reasons.join('; ')}.`)
  }
  const warnings = score.warnings.slice(0, 6)
  if (warnings.length > 0) {
    parts.push(`Warnings: ${warnings.join('; ')}.`)
  }
  const failed = score.failed_filters.slice(0, 5)
  if (failed.length > 0) {
    parts.push(`Failed filters: ${failed.join('; ')}.`)
  }
  let s = parts.join(' ')
  if (s.length > 7900) {
    s = `${s.slice(0, 7900)}…`
  }
  return s
}

function pctChange24h(t: MarketTicker): number {
  if (t.open_24h === 0) return 0
  return ((t.price - t.open_24h) / t.open_24h) * 100
}

function spreadPct(t: MarketTicker): number {
  const mid = (t.best_bid + t.best_ask) / 2
  if (mid === 0) return 0
  return ((t.best_ask - t.best_bid) / mid) * 100
}

async function fetchMarketTickers(): Promise<MarketTicker[]> {
  const res = await fetch(apiUrl('/api/v1/market/tickers'))
  if (!res.ok) {
    throw new Error(await readApiError(res))
  }
  const data: unknown = await res.json()
  if (!Array.isArray(data)) {
    throw new Error('Unexpected ticker list response')
  }
  return data as MarketTicker[]
}

type OmrManualAnalysisCardProps = {
  productId: string
  price: number
  formatNum: (n: number) => string
}

function OmrManualAnalysisCard({
  productId,
  price,
  formatNum,
}: OmrManualAnalysisCardProps) {
  const accessToken = useAuthStore((s) => s.auth.accessToken)

  const [rsi, setRsi] = useState('38')
  const [supportDistancePercent, setSupportDistancePercent] = useState('1.5')
  const [volumeTrend, setVolumeTrend] =
    useState<(typeof VOLUME_TRENDS)[number]>('rising')
  const [candleSignal, setCandleSignal] =
    useState<(typeof CANDLE_SIGNALS)[number]>('bullish_reversal')
  const [higherLowDetected, setHigherLowDetected] = useState(true)
  const [plannedEntry, setPlannedEntry] = useState(() => String(price))
  const [stopLoss, setStopLoss] = useState(() => String(price * 0.97))
  const [targetPrice, setTargetPrice] = useState(() => String(price * 1.09))
  const [positionSize, setPositionSize] = useState('1')
  const [planNotes, setPlanNotes] = useState('')

  const [scoreResult, setScoreResult] = useState<OmrScoreResult | null>(null)
  const [scoreError, setScoreError] = useState<string | null>(null)
  const [scoreSubmitting, setScoreSubmitting] = useState(false)
  const [saveSubmitting, setSaveSubmitting] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveSuccessId, setSaveSuccessId] = useState<string | null>(null)

  async function submitScore() {
    setScoreError(null)
    setScoreResult(null)
    setSaveError(null)
    setSaveSuccessId(null)
    const rsiNum = Number.parseFloat(rsi)
    const supNum = Number.parseFloat(supportDistancePercent)
    const entryNum = Number.parseFloat(plannedEntry)
    const stopNum = Number.parseFloat(stopLoss)
    const targetNum = Number.parseFloat(targetPrice)
    if (
      Number.isNaN(rsiNum) ||
      Number.isNaN(supNum) ||
      Number.isNaN(entryNum) ||
      Number.isNaN(stopNum) ||
      Number.isNaN(targetNum)
    ) {
      setScoreError('Enter valid numbers for all numeric fields.')
      return
    }

    const body = {
      rsi: rsiNum,
      support_distance_percent: supNum,
      volume_trend: volumeTrend,
      candle_signal: candleSignal,
      higher_low_detected: higherLowDetected,
      planned_entry: entryNum,
      stop_loss: stopNum,
      target_price: targetNum,
    }

    setScoreSubmitting(true)
    try {
      const encoded = encodeURIComponent(productId)
      const res = await fetch(
        apiUrl(
          `/api/v1/market/tickers/${encoded}/overnight-mean-reversion/score`,
        ),
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        },
      )
      if (!res.ok) {
        setScoreError(await readApiError(res))
        return
      }
      const data: unknown = await res.json()
      setScoreResult(data as OmrScoreResult)
    } catch (e) {
      setScoreError(e instanceof Error ? e.message : 'Request failed')
    } finally {
      setScoreSubmitting(false)
    }
  }

  async function saveTradePlan() {
    setSaveError(null)
    setSaveSuccessId(null)
    if (!scoreResult) return

    if (!accessToken?.trim()) {
      setSaveError(
        'Sign in with a Chexi account and obtain an access token to save trade plans (POST /api/v1/trade-plans requires authentication).',
      )
      return
    }

    const pos = Number.parseFloat(positionSize)
    if (Number.isNaN(pos) || pos <= 0) {
      setSaveError('Enter a valid position size greater than zero.')
      return
    }

    const entryNum = Number.parseFloat(plannedEntry)
    const stopNum = Number.parseFloat(stopLoss)
    const targetNum = Number.parseFloat(targetPrice)
    if (
      Number.isNaN(entryNum) ||
      Number.isNaN(stopNum) ||
      Number.isNaN(targetNum)
    ) {
      setSaveError(
        'Planned entry, stop loss, and target price must be valid numbers.',
      )
      return
    }

    if (!(stopNum < entryNum && entryNum < targetNum)) {
      setSaveError(
        'Cannot save: LONG trade plans require stop_loss < planned_entry < target_price. Adjust levels or fix geometry.',
      )
      return
    }

    const riskPerUnit = entryNum - stopNum
    const maxRiskAmount = riskPerUnit * pos
    if (riskPerUnit <= 0 || maxRiskAmount <= 0) {
      setSaveError(
        'Cannot save: risk per unit and max risk amount must be positive.',
      )
      return
    }

    const payload = {
      symbol: productId,
      strategy_name: 'overnight_mean_reversion',
      direction: 'LONG',
      thesis: buildThesisFromScore(scoreResult),
      planned_entry: entryNum,
      stop_loss: stopNum,
      target_price: targetNum,
      position_size: pos,
      max_risk_amount: maxRiskAmount,
      source_score: scoreResult.final_score,
      source_label: scoreResult.label,
      notes: planNotes.trim(),
    }

    setSaveSubmitting(true)
    try {
      const res = await fetch(apiUrl('/api/v1/trade-plans'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken}`,
        },
        body: JSON.stringify(payload),
      })
      if (!res.ok) {
        setSaveError(await readApiError(res))
        return
      }
      const data: unknown = await res.json()
      const id =
        data &&
        typeof data === 'object' &&
        'id' in data &&
        typeof (data as { id: unknown }).id === 'string'
          ? (data as { id: string }).id
          : null
      if (!id) {
        setSaveError('Save succeeded but response did not include an id.')
        return
      }
      setSaveSuccessId(id)
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Save request failed')
    } finally {
      setSaveSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Manual analysis</CardTitle>
        <CardDescription>
          POST to{' '}
          <code className='text-xs'>
            /api/v1/market/tickers/:productID/overnight-mean-reversion/score
          </code>
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
          <div className='space-y-2'>
            <Label htmlFor='rsi'>RSI</Label>
            <Input
              id='rsi'
              inputMode='decimal'
              value={rsi}
              onChange={(e) => setRsi(e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='support'>Support distance %</Label>
            <Input
              id='support'
              inputMode='decimal'
              value={supportDistancePercent}
              onChange={(e) => setSupportDistancePercent(e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label>Volume trend</Label>
            <Select
              value={volumeTrend}
              onValueChange={(v) =>
                setVolumeTrend(v as (typeof VOLUME_TRENDS)[number])
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {VOLUME_TRENDS.map((v) => (
                  <SelectItem key={v} value={v}>
                    {v}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-2'>
            <Label>Candle signal</Label>
            <Select
              value={candleSignal}
              onValueChange={(v) =>
                setCandleSignal(v as (typeof CANDLE_SIGNALS)[number])
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CANDLE_SIGNALS.map((v) => (
                  <SelectItem key={v} value={v}>
                    {v}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='flex items-end gap-2 pb-2'>
            <Checkbox
              id='higher-low'
              checked={higherLowDetected}
              onCheckedChange={(c) => setHigherLowDetected(c === true)}
            />
            <Label htmlFor='higher-low' className='cursor-pointer'>
              Higher low detected
            </Label>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='entry'>Planned entry</Label>
            <Input
              id='entry'
              inputMode='decimal'
              value={plannedEntry}
              onChange={(e) => setPlannedEntry(e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='stop'>Stop loss</Label>
            <Input
              id='stop'
              inputMode='decimal'
              value={stopLoss}
              onChange={(e) => setStopLoss(e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='target'>Target price</Label>
            <Input
              id='target'
              inputMode='decimal'
              value={targetPrice}
              onChange={(e) => setTargetPrice(e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='position-size'>Position size (units)</Label>
            <Input
              id='position-size'
              inputMode='decimal'
              value={positionSize}
              onChange={(e) => setPositionSize(e.target.value)}
            />
            <p className='text-muted-foreground text-xs'>
              Used when saving a trade plan; for LONG, max risk sent is (planned
              entry − stop) × size.
            </p>
          </div>
        </div>

        <Button
          type='button'
          onClick={() => void submitScore()}
          disabled={scoreSubmitting}
        >
          {scoreSubmitting ? (
            <>
              <Loader2 className='me-2 h-4 w-4 animate-spin' />
              Scoring…
            </>
          ) : (
            'Run OMR score'
          )}
        </Button>

        {scoreError ? (
          <Alert variant='destructive'>
            <AlertTitle>Score request failed</AlertTitle>
            <AlertDescription>{scoreError}</AlertDescription>
          </Alert>
        ) : null}

        {scoreResult ? (
          <div className='space-y-4 border-t pt-4'>
            <div className='flex flex-wrap items-baseline gap-3'>
              <div>
                <div className='text-muted-foreground text-sm'>
                  Final score
                </div>
                <div className='text-2xl font-semibold tabular-nums'>
                  {formatNum(scoreResult.final_score)}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground text-sm'>Label</div>
                <div className='font-medium'>{scoreResult.label}</div>
              </div>
            </div>

            <div>
              <h3 className='mb-2 text-sm font-medium'>
                Category scores (0–100)
              </h3>
              <ul className='text-muted-foreground grid gap-1 text-sm sm:grid-cols-2'>
                <li>
                  Liquidity:{' '}
                  <span className='text-foreground font-mono'>
                    {formatNum(scoreResult.category_scores.liquidity)}
                  </span>
                </li>
                <li>
                  Trend quality:{' '}
                  <span className='text-foreground font-mono'>
                    {formatNum(scoreResult.category_scores.trend_quality)}
                  </span>
                </li>
                <li>
                  Pullback quality:{' '}
                  <span className='text-foreground font-mono'>
                    {formatNum(scoreResult.category_scores.pullback_quality)}
                  </span>
                </li>
                <li>
                  Exhaustion:{' '}
                  <span className='text-foreground font-mono'>
                    {formatNum(scoreResult.category_scores.exhaustion)}
                  </span>
                </li>
                <li>
                  Risk setup:{' '}
                  <span className='text-foreground font-mono'>
                    {formatNum(scoreResult.category_scores.risk_setup)}
                  </span>
                </li>
                <li>
                  Final reversal:{' '}
                  <span className='text-foreground font-mono'>
                    {formatNum(scoreResult.category_scores.final_reversal)}
                  </span>
                </li>
              </ul>
            </div>

            {scoreResult.reasons.length > 0 ? (
              <div>
                <h3 className='mb-2 text-sm font-medium'>Reasons</h3>
                <ul className='list-inside list-disc text-sm'>
                  {scoreResult.reasons.map((r) => (
                    <li key={r}>{r}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            {scoreResult.warnings.length > 0 ? (
              <div>
                <h3 className='mb-2 text-sm font-medium'>Warnings</h3>
                <ul className='list-inside list-disc text-sm text-amber-700 dark:text-amber-400'>
                  {scoreResult.warnings.map((w) => (
                    <li key={w}>{w}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            {scoreResult.failed_filters.length > 0 ? (
              <div>
                <h3 className='mb-2 text-sm font-medium'>Failed filters</h3>
                <ul className='list-inside list-disc text-sm'>
                  {scoreResult.failed_filters.map((f) => (
                    <li key={f}>{f}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            <div className='space-y-2'>
              <Label htmlFor='plan-notes'>
                Notes (optional, saved on trade plan)
              </Label>
              <Textarea
                id='plan-notes'
                rows={3}
                className='resize-y'
                placeholder='Context, execution reminders, or links…'
                value={planNotes}
                onChange={(e) => setPlanNotes(e.target.value)}
              />
            </div>

            <div className='flex flex-wrap items-center gap-2'>
              <Button
                type='button'
                variant='secondary'
                onClick={() => void saveTradePlan()}
                disabled={!scoreResult || saveSubmitting}
              >
                {saveSubmitting ? (
                  <>
                    <Loader2 className='me-2 h-4 w-4 animate-spin' />
                    Saving…
                  </>
                ) : (
                  'Save as Trade Plan'
                )}
              </Button>
            </div>

            {saveError ? (
              <Alert variant='destructive'>
                <AlertTitle>Could not save trade plan</AlertTitle>
                <AlertDescription>{saveError}</AlertDescription>
              </Alert>
            ) : null}

            {saveSuccessId ? (
              <Alert>
                <AlertTitle>Trade plan saved</AlertTitle>
                <AlertDescription>
                  Created trade plan id:{' '}
                  <code className='text-xs'>{saveSuccessId}</code>. This remains
                  advisory only — no orders were placed.
                </AlertDescription>
              </Alert>
            ) : null}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

export function OmrScannerPage() {
  const {
    data: tickers,
    isLoading,
    isError,
    error,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ['market-tickers'],
    queryFn: fetchMarketTickers,
    refetchInterval: 5_000,
  })

  const sortedTickers = useMemo(() => {
    if (!tickers?.length) return []
    return [...tickers].sort((a, b) =>
      a.product_id.localeCompare(b.product_id),
    )
  }, [tickers])

  const [userProductId, setUserProductId] = useState<string | null>(null)

  const activeProductId = useMemo(() => {
    if (sortedTickers.length === 0) return ''
    const userOk =
      userProductId &&
      sortedTickers.some((t) => t.product_id === userProductId)
    if (userOk) return userProductId as string
    return sortedTickers[0].product_id
  }, [sortedTickers, userProductId])

  const selectedTicker = sortedTickers.find(
    (t) => t.product_id === activeProductId,
  )

  const nf = useMemo(
    () =>
      new Intl.NumberFormat(undefined, {
        maximumFractionDigits: 2,
        minimumFractionDigits: 0,
      }),
    [],
  )

  const formatNum = (n: number) => nf.format(n)

  const volCompact = useMemo(
    () =>
      new Intl.NumberFormat(undefined, {
        notation: 'compact',
        maximumFractionDigits: 1,
      }),
    [],
  )

  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div>
          <h1 className='text-2xl font-bold tracking-tight'>
            Overnight Mean Reversion Scanner
          </h1>
          <p className='text-muted-foreground'>
            Live market tickers plus manual analysis inputs. Scores are computed
            on the server from the latest in-memory ticker for the selected
            product.
          </p>
        </div>

        <Alert>
          <AlertCircle className='h-4 w-4' />
          <AlertTitle>Advisory only</AlertTitle>
          <AlertDescription>
            Advisory only — no automatic trading. This page validates the
            scanner pipeline; scoring does not place orders. Saving a trade plan
            stores an advisory record via the API only — it still does not trade.
          </AlertDescription>
        </Alert>

        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void refetch()}
            disabled={isFetching}
          >
            {isFetching ? (
              <>
                <Loader2 className='me-2 h-4 w-4 animate-spin' />
                Refreshing…
              </>
            ) : (
              'Refresh tickers'
            )}
          </Button>
          {isFetching && !isLoading ? (
            <span className='text-muted-foreground text-sm'>
              Updating list…
            </span>
          ) : null}
        </div>

        {isError ? (
          <Alert variant='destructive'>
            <AlertTitle>Could not load tickers</AlertTitle>
            <AlertDescription>
              {error instanceof Error ? error.message : 'Unknown error'}
            </AlertDescription>
          </Alert>
        ) : null}

        <Card>
          <CardHeader>
            <CardTitle>Live tickers</CardTitle>
            <CardDescription>
              From{' '}
              <code className='text-xs'>GET /api/v1/market/tickers</code>. When
              the store is empty, connect the Go API to Coinbase or seed
              tickers for testing.
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            {isLoading ? (
              <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                <Loader2 className='h-4 w-4 animate-spin' />
                Loading tickers…
              </div>
            ) : sortedTickers.length === 0 ? (
              <Alert>
                <AlertTitle>No tickers yet</AlertTitle>
                <AlertDescription>
                  The in-memory store has no products. Start the API with live
                  ticker ingestion, or seed data for manual checks.
                </AlertDescription>
              </Alert>
            ) : (
              <div className='rounded-md border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Product</TableHead>
                      <TableHead className='text-end'>Price</TableHead>
                      <TableHead className='text-end'>24h vol</TableHead>
                      <TableHead className='text-end'>24h chg %</TableHead>
                      <TableHead className='text-end'>Spread %</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sortedTickers.map((t) => {
                      const chg = pctChange24h(t)
                      const sp = spreadPct(t)
                      const active = t.product_id === activeProductId
                      return (
                        <TableRow
                          key={t.product_id}
                          data-state={active ? 'selected' : undefined}
                          className={cn(
                            'cursor-pointer',
                            active && 'bg-muted/60',
                          )}
                          onClick={() => setUserProductId(t.product_id)}
                        >
                          <TableCell className='font-medium'>
                            {t.product_id}
                          </TableCell>
                          <TableCell className='text-end'>
                            {nf.format(t.price)}
                          </TableCell>
                          <TableCell className='text-end'>
                            {volCompact.format(t.volume_24h)}
                          </TableCell>
                          <TableCell
                            className={cn(
                              'text-end tabular-nums',
                              chg > 0 && 'text-emerald-600 dark:text-emerald-400',
                              chg < 0 && 'text-red-600 dark:text-red-400',
                            )}
                          >
                            {nf.format(chg)}%
                          </TableCell>
                          <TableCell className='text-end tabular-nums'>
                            {nf.format(sp)}%
                          </TableCell>
                        </TableRow>
                      )
                    })}
                  </TableBody>
                </Table>
              </div>
            )}

            {sortedTickers.length > 0 ? (
              <div className='max-w-xs space-y-2'>
                <Label htmlFor='product-select'>Selected product</Label>
                <Select
                  value={activeProductId}
                  onValueChange={setUserProductId}
                >
                  <SelectTrigger id='product-select'>
                    <SelectValue placeholder='Choose product' />
                  </SelectTrigger>
                  <SelectContent>
                    {sortedTickers.map((t) => (
                      <SelectItem key={t.product_id} value={t.product_id}>
                        {t.product_id}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            ) : null}
          </CardContent>
        </Card>

        {activeProductId && selectedTicker ? (
          <OmrManualAnalysisCard
            key={activeProductId}
            productId={activeProductId}
            price={selectedTicker.price}
            formatNum={formatNum}
          />
        ) : null}
      </Main>
    </>
  )
}
