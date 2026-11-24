import type { components } from './schema'

type Schemas = components['schemas']

export type Reading = Schemas['Reading']
export type ReadingInput = Schemas['ReadingInput']
export type Token = Schemas['Token']
export type TokenCreate = Schemas['TokenCreate']
export type TokenUpsert = Schemas['TokenUpsert']
export type TokenPatch = Schemas['TokenPatch']
export type TokenMarkUsed = Schemas['TokenMarkUsed']
export type TokenMarkUsedResult = Schemas['TokenMarkUsedResult']
export type TokenCancelResult = Schemas['TokenCancelResult']
export type Settings = Schemas['Settings']
export type SettingsPatch = Schemas['SettingsPatch']
export type Tariff = Schemas['Tariff']
export type Summary = Schemas['Summary']
export type UsageBucket = Schemas['UsageBucket']
export type Recommendation = Schemas['Recommendation']
export type TariffComparison = Schemas['TariffComparison']
export type Urgency = Schemas['Urgency']
export type Alert = Schemas['Alert']
export type SessionState = Schemas['SessionState']
export type ImportPreview = Schemas['ImportPreview']
export type ImportCommitResult = Schemas['ImportCommitResult']
export type PushSubscription = Schemas['PushSubscription']
export type PushSubscriptionInput = Schemas['PushSubscriptionInput']
export type Homepage = Schemas['Homepage']
export type PageMeta = Schemas['PageMeta']

export type ErrorBody = Schemas['Error']['error']
export type ErrorCode = ErrorBody['code']
export type ErrorDetail = NonNullable<ErrorBody['details']>[number]

export interface Page<T> {
  data: T[]
  meta: PageMeta
}