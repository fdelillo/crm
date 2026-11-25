import { FormAlert } from '@/components/FormAlert'

export interface ReadOnlyNoticeProps {
  text: string
}

// T-002-F103 (002/BR-F1): the read-only views of the Operator say so in plain text, not announced.
export function ReadOnlyNotice({ text }: ReadOnlyNoticeProps) {
  return <FormAlert variant="info" announce={false} title={text} />
}
