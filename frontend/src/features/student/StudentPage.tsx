import { CheckIcon, Loader2Icon, LockIcon } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { ApiError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { RegistrationConfirmation } from '@/lib/types'
import { useRegister, useRegistrationStatus, useTopics } from './api'
import { ConfirmationView } from './ConfirmationView'
import { TopicCard } from './TopicCard'
import {
  hasErrors,
  normalizeIndexNumber,
  normalizeName,
  validateStudent,
  type FieldErrors,
} from './validate'

const STORAGE_KEY = 'groupify:confirmation'

type Step = 'form' | 'review'

function readStoredConfirmation(): RegistrationConfirmation | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (
      typeof parsed === 'object' &&
      parsed !== null &&
      'registration_id' in parsed &&
      typeof (parsed as RegistrationConfirmation).registration_id === 'number'
    ) {
      return parsed as RegistrationConfirmation
    }
    return null
  } catch {
    return null
  }
}

export default function StudentPage() {
  const topicsQuery = useTopics()
  const statusQuery = useRegistrationStatus()
  const register = useRegister()

  const [confirmation, setConfirmation] = useState<RegistrationConfirmation | null>(
    readStoredConfirmation,
  )
  const [step, setStep] = useState<Step>('form')
  const [fullName, setFullName] = useState('')
  const [indexNumber, setIndexNumber] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [reviewError, setReviewError] = useState<string | null>(null)

  // Reused across retries of the same payload; replaced whenever the student
  // goes back and edits, because the body may differ.
  const idempotencyKey = useRef<string>(crypto.randomUUID())
  const panelRef = useRef<HTMLDivElement>(null)
  const panelHeadingRef = useRef<HTMLHeadingElement>(null)

  const topics = topicsQuery.data?.topics ?? []
  const registrationOpen = statusQuery.data?.registration_open ?? false
  const selectedTopic = topics.find((topic) => topic.id === selectedId) ?? null

  useEffect(() => {
    if (step !== 'review') return
    if (typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches) {
      panelRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
    panelHeadingRef.current?.focus()
  }, [step])

  function handleSelect(id: number) {
    setSelectedId(id)
    setFormError(null)
    // On a phone the form sits below the list, so bring it into view rather
    // than making the student hunt for it.
    if (window.matchMedia('(max-width: 1023px)').matches) {
      panelRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }

  function handleReview(event: React.FormEvent) {
    event.preventDefault()
    setFormError(null)

    const errors = validateStudent(fullName, indexNumber)
    setFieldErrors(errors)

    if (hasErrors(errors)) {
      const firstInvalid = Object.keys(errors)[0]
      document.getElementById(firstInvalid === 'full_name' ? 'full-name' : 'index-number')?.focus()
      return
    }
    if (!selectedTopic) {
      setFormError('Choose a topic before continuing.')
      return
    }
    if (selectedTopic.status === 'full') {
      setFormError('That topic is full. Please choose another available topic.')
      void topicsQuery.refetch()
      return
    }

    idempotencyKey.current = crypto.randomUUID()
    setReviewError(null)
    setStep('review')
  }

  function handleEdit() {
    setStep('form')
    setReviewError(null)
    panelHeadingRef.current?.focus()
  }

  async function handleConfirm() {
    if (!selectedTopic) return
    setReviewError(null)

    try {
      const result = await register.mutateAsync({
        values: {
          full_name: normalizeName(fullName),
          index_number: normalizeIndexNumber(indexNumber),
          topic_id: selectedTopic.id,
        },
        idempotencyKey: idempotencyKey.current,
      })

      try {
        sessionStorage.setItem(STORAGE_KEY, JSON.stringify(result))
      } catch {
        // Private mode can refuse storage; the in-memory confirmation is enough.
      }
      setConfirmation(result)
    } catch (error) {
      handleSubmitError(error)
    }
  }

  function handleSubmitError(error: unknown) {
    const message = error instanceof ApiError ? error.message : 'Something went wrong.'

    if (!(error instanceof ApiError)) {
      setReviewError('Could not reach the server. Check your connection and try again.')
      return
    }

    switch (error.code) {
      case 'topic_full':
        // Somebody took the last seat between review and confirm.
        setStep('form')
        setSelectedId(null)
        setFormError(
          `${selectedTopic?.name ?? 'That topic'} filled up while you were reviewing. Choose another available topic.`,
        )
        void topicsQuery.refetch()
        break
      case 'registration_closed':
        setStep('form')
        setFormError('Registration closed while you were filling in the form. No place was reserved.')
        void statusQuery.refetch()
        break
      case 'topic_inactive':
      case 'topic_not_found':
        setStep('form')
        setSelectedId(null)
        setFormError('That topic is no longer available. Choose another one.')
        void topicsQuery.refetch()
        break
      case 'index_number_already_registered':
        setStep('form')
        setFieldErrors({ index_number: 'This index number is already registered.' })
        setFormError('Each student can register only once. If this is a mistake, contact your course rep.')
        break
      case 'validation_failed':
        setStep('form')
        setFieldErrors((error.fields ?? {}) as FieldErrors)
        setFormError(error.message)
        break
      case 'rate_limited':
        setReviewError('Too many attempts in a short time. Wait a minute, then try again.')
        break
      case 'network_error':
        setReviewError('Could not reach the server. Your place is not reserved yet — try again.')
        break
      default:
        setReviewError(message)
    }
  }

  if (confirmation) {
    return <ConfirmationView confirmation={confirmation} />
  }

  const loading = topicsQuery.isLoading || statusQuery.isLoading
  const loadFailed = topicsQuery.isError || statusQuery.isError

  return (
    <div className="min-h-dvh bg-background">
      <div className="border-b bg-linear-to-b from-muted/70 to-background">
        <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 sm:py-14">
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">Choose your topic</h1>
          <p className="mt-2 max-w-2xl text-muted-foreground">
            Pick exactly one topic from the list. Your place is reserved when you confirm, and your
            choice is final.
          </p>

          <div className="mt-6">
            {statusQuery.isLoading ? (
              <Skeleton className="h-6 w-44 rounded-full" />
            ) : (
              <span
                className={cn(
                  'inline-flex items-center gap-2 rounded-full border px-3 py-1 text-sm',
                  registrationOpen
                    ? 'border-emerald-600/25 bg-emerald-500/10 text-emerald-800 dark:text-emerald-300'
                    : 'border-border bg-muted text-muted-foreground',
                )}
              >
                <span
                  className={cn(
                    'size-1.5 rounded-full',
                    registrationOpen ? 'bg-emerald-600' : 'bg-muted-foreground',
                  )}
                  aria-hidden="true"
                />
                {registrationOpen ? 'Registration is open' : 'Registration is closed'}
              </span>
            )}
          </div>
        </div>
      </div>

      <div className="mx-auto grid max-w-6xl gap-8 px-4 py-8 sm:px-6 lg:grid-cols-[minmax(0,1fr)_24rem] lg:py-12">
        <section aria-labelledby="topics-heading" className="min-w-0">
          <div className="flex items-baseline justify-between gap-3">
            <h2 id="topics-heading" className="text-lg font-semibold">
              Available topics
            </h2>
            {!loading && topics.length > 0 && (
              <span className="text-xs text-muted-foreground">
                {topics.length} {topics.length === 1 ? 'topic' : 'topics'}
              </span>
            )}
          </div>

          {loading && (
            <div className="mt-3 space-y-3">
              {[0, 1, 2].map((index) => (
                <Skeleton key={index} className="h-32 rounded-xl" />
              ))}
            </div>
          )}

          {loadFailed && (
            <Alert variant="destructive" className="mt-3">
              <AlertTitle>Could not load topics</AlertTitle>
              <AlertDescription>
                <p>The list is unavailable right now.</p>
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-2"
                  onClick={() => {
                    void topicsQuery.refetch()
                    void statusQuery.refetch()
                  }}
                >
                  Try again
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {!loading && !loadFailed && topics.length === 0 && (
            <p className="mt-3 rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
              No topics have been published yet. Check back once your course rep adds them.
            </p>
          )}

          {!loading && topics.length > 0 && (
            <fieldset className="mt-3" disabled={!registrationOpen || register.isPending}>
              <legend className="sr-only">Choose exactly one topic</legend>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
                {topics.map((topic) => (
                  <TopicCard
                    key={topic.id}
                    topic={topic}
                    selected={topic.id === selectedId}
                    disabled={!registrationOpen}
                    onSelect={handleSelect}
                  />
                ))}
              </div>
            </fieldset>
          )}
        </section>

        <div ref={panelRef} className="lg:sticky lg:top-6 lg:self-start">
          <Card>
            <CardHeader>
              <CardTitle>
                <h2
                  ref={panelHeadingRef}
                  tabIndex={-1}
                  className="text-base font-semibold outline-none"
                >
                  {step === 'form' ? 'Your details' : 'Check and confirm'}
                </h2>
              </CardTitle>
              <CardDescription>
                {step === 'form'
                  ? 'Use your name and index number as they appear on the student register.'
                  : 'One last look before your place is reserved.'}
              </CardDescription>
            </CardHeader>

            <CardContent>
              {step === 'form' ? (
                <form onSubmit={handleReview} noValidate className="space-y-4">
                  {!registrationOpen && statusQuery.isSuccess && (
                    <Alert>
                      <LockIcon />
                      <AlertTitle>Registration is closed</AlertTitle>
                      <AlertDescription>
                        You can browse the topics, but new registrations are not being accepted.
                      </AlertDescription>
                    </Alert>
                  )}

                  {formError && (
                    <Alert variant="destructive">
                      <AlertTitle>Could not continue</AlertTitle>
                      <AlertDescription>{formError}</AlertDescription>
                    </Alert>
                  )}

                  <div className="space-y-1.5">
                    <Label htmlFor="full-name">Full name</Label>
                    <Input
                      id="full-name"
                      name="fullName"
                      autoComplete="name"
                      className="h-10"
                      placeholder="Ama Mensah"
                      value={fullName}
                      onChange={(event) => setFullName(event.target.value)}
                      aria-invalid={Boolean(fieldErrors.full_name)}
                      aria-describedby={fieldErrors.full_name ? 'full-name-error' : undefined}
                    />
                    {fieldErrors.full_name && (
                      <p id="full-name-error" className="text-xs text-destructive">
                        {fieldErrors.full_name}
                      </p>
                    )}
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="index-number">Index number</Label>
                    <Input
                      id="index-number"
                      name="indexNumber"
                      autoComplete="off"
                      autoCapitalize="characters"
                      spellCheck={false}
                      className="h-10 font-mono"
                      placeholder="CS/2024/045"
                      value={indexNumber}
                      onChange={(event) => setIndexNumber(event.target.value)}
                      aria-invalid={Boolean(fieldErrors.index_number)}
                      aria-describedby={
                        fieldErrors.index_number ? 'index-number-error' : 'index-number-hint'
                      }
                    />
                    {fieldErrors.index_number ? (
                      <p id="index-number-error" className="text-xs text-destructive">
                        {fieldErrors.index_number}
                      </p>
                    ) : (
                      <p id="index-number-hint" className="text-xs text-muted-foreground">
                        {normalizeIndexNumber(indexNumber) &&
                        normalizeIndexNumber(indexNumber) !== indexNumber.trim() ? (
                          <>
                            Saved as{' '}
                            <span className="font-mono text-foreground">
                              {normalizeIndexNumber(indexNumber)}
                            </span>
                          </>
                        ) : (
                          'Letters, digits and / - . _ only.'
                        )}
                      </p>
                    )}
                  </div>

                  <div className="rounded-lg bg-muted/60 p-3 text-sm">
                    <p className="text-xs uppercase tracking-wide text-muted-foreground">
                      Selected topic
                    </p>
                    <p className="mt-1 font-medium">
                      {selectedTopic ? selectedTopic.name : 'None yet'}
                    </p>
                  </div>

                  <Button type="submit" size="lg" className="h-11 w-full" disabled={!registrationOpen}>
                    Review selection
                  </Button>
                </form>
              ) : (
                <div className="space-y-4">
                  {reviewError && (
                    <Alert variant="destructive">
                      <AlertTitle>Not registered</AlertTitle>
                      <AlertDescription>{reviewError}</AlertDescription>
                    </Alert>
                  )}

                  <dl className="divide-y text-sm">
                    <div className="flex justify-between gap-4 py-2.5">
                      <dt className="text-muted-foreground">Name</dt>
                      <dd className="text-right font-medium">{normalizeName(fullName)}</dd>
                    </div>
                    <div className="flex justify-between gap-4 py-2.5">
                      <dt className="text-muted-foreground">Index number</dt>
                      <dd className="text-right font-mono font-medium">
                        {normalizeIndexNumber(indexNumber)}
                      </dd>
                    </div>
                    <div className="flex justify-between gap-4 py-2.5">
                      <dt className="text-muted-foreground">Topic</dt>
                      <dd className="text-right font-medium">{selectedTopic?.name}</dd>
                    </div>
                  </dl>

                  <Alert>
                    <CheckIcon />
                    <AlertTitle>This choice is final</AlertTitle>
                    <AlertDescription>
                      After you confirm, you cannot switch or cancel it yourself. Your course rep can
                      correct mistakes.
                    </AlertDescription>
                  </Alert>

                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="lg"
                      className="h-11 flex-1"
                      onClick={handleEdit}
                      disabled={register.isPending}
                    >
                      Back
                    </Button>
                    <Button
                      size="lg"
                      className="h-11 flex-[1.4]"
                      onClick={handleConfirm}
                      disabled={register.isPending}
                    >
                      {register.isPending ? (
                        <>
                          <Loader2Icon className="animate-spin" aria-hidden="true" />
                          Reserving…
                        </>
                      ) : (
                        'Confirm registration'
                      )}
                    </Button>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          <p className="mt-3 px-1 text-xs text-muted-foreground">
            Only topic names and remaining places are public. Nobody can see who else registered.
          </p>
        </div>
      </div>
    </div>
  )
}
