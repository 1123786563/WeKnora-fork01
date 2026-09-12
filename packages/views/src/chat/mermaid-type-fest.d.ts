declare module 'type-fest' {
  export type SetOptional<BaseType, Keys extends keyof BaseType = keyof BaseType> = Omit<BaseType, Keys> & Partial<Pick<BaseType, Keys>>;
  export type SetRequired<BaseType, Keys extends keyof BaseType = keyof BaseType> = Omit<BaseType, Keys> & Required<Pick<BaseType, Keys>>;
}
