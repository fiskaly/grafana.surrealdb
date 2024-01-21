import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
    const { jsonData, secureJsonFields } = options;
  const secureJsonData = (options.secureJsonData || {}) as MySecureJsonData;
   
  return (
    <div className="gf-form-group">
      <InlineField
        label="Location"
        labelWidth={14}
        tooltip="Websocket location in the format `address:port`."
      >
        <Input
          value={jsonData.location || ''}
          placeholder="localhost:8000"
          width={40}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
	      onOptionsChange({
		  ...options,
		  jsonData: {
		      location: event.target.value,
		  },
	      });
	  }}
        />
      </InlineField>
      <InlineField
        label="Namespace"
        labelWidth={14}
        tooltip="Namespace used for the signin operation."
      >
        <Input
          value={jsonData.nameaddr || ''}
          placeholder="default"
          width={40}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
	      onOptionsChange({
		  ...options,
		  jsonData: {
		      ...options.jsonData,
		      nameaddr: event.target.value,
		  },
	      });
	  }}
        />
      </InlineField>
      <InlineField
        label="Database"
        labelWidth={14}
        tooltip="Database used for the signin operation."
      >
        <Input
          value={jsonData.database || ''}
          placeholder="default"
          width={40}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
	      onOptionsChange({
		  ...options,
		  jsonData: {
		      ...options.jsonData,
		      database: event.target.value,
		  },
	      });
	  }}
        />
      </InlineField>
      <InlineField
        label="Scope"
        labelWidth={14}
        tooltip="Optional scope used for the signin operation."
      >
        <Input
          value={jsonData.scope || ''}
          placeholder=""
          width={40}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
	      onOptionsChange({
		  ...options,
		  jsonData: {
		      ...options.jsonData,
		      scope: event.target.value,
		  },
	      });
	  }}
        />
      </InlineField>
      <InlineField
        label="Username"
        labelWidth={14}
        tooltip="User used for the signin operation."
      >
        <Input
          value={jsonData.username || ''}
          placeholder="username"
          width={40}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
	      onOptionsChange({
		  ...options,
		  jsonData: {
		      ...options.jsonData,
		      username: event.target.value,
		  },
	      });
	  }}
        />
      </InlineField>
      <InlineField
        label="Password"
        labelWidth={14}
        tooltip="Password used for the signin operation."
      >
        <SecretInput
          value={secureJsonData.password || ''}
          placeholder="password"
          width={40}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
	      onOptionsChange({
		  ...options,
		  secureJsonData: {
		      ...options.secureJsonData,
		      password: event.target.value,
		  },
	      });
	  }}
          onReset={() => {
	    onOptionsChange({
	      ...options,
	      secureJsonFields: {
	        ...options.secureJsonFields,
	        password: false,
	      },
	      secureJsonData: {
	        ...options.secureJsonData,
	        password: '',
	      },
            });
          }}
          isConfigured={(secureJsonFields && secureJsonFields.password) as boolean}
        />
      </InlineField>
    </div>
  );
}
